// Package pdp is the billing policy-decision point: the can-use / report /
// authorize logic + the local HTTP API products call. It caches entitlements
// (stale-on-error) and applies the fail-open/closed policy.
package pdp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/simplify/billing-agent/internal/upstream"
)

type Agent struct {
	up       *upstream.Client
	cacheTTL time.Duration
	failOpen bool
	batcher  reporter // optional; batches the report() write path

	mu  sync.RWMutex
	ent map[string]cached
}

// reporter is the batch sink for free-quota report() events (implemented by
// internal/batch.Batcher). authorize never goes through it.
type reporter interface {
	Add(ev upstream.BatchEvent)
	Enabled() bool
}

// SetBatcher enables batched consumption write-back for report().
func (a *Agent) SetBatcher(b reporter) { a.batcher = b }

// Invalidate drops cached snapshots for the given orgs (called after a batch flush).
func (a *Agent) Invalidate(orgs []string) {
	a.mu.Lock()
	for _, o := range orgs {
		delete(a.ent, o)
	}
	a.mu.Unlock()
}

type cached struct {
	at  time.Time
	val upstream.Entitlement
}

func New(up *upstream.Client, cacheTTL time.Duration, failOpen bool) *Agent {
	return &Agent{up: up, cacheTTL: cacheTTL, failOpen: failOpen, ent: map[string]cached{}}
}

// entitlement returns the org snapshot cache-first, with stale-on-error.
func (a *Agent) entitlement(ctx context.Context, org string) (upstream.Entitlement, error) {
	a.mu.RLock()
	c, ok := a.ent[org]
	a.mu.RUnlock()
	if ok && time.Since(c.at) < a.cacheTTL {
		return c.val, nil
	}
	ent, status, err := a.up.GetEntitlement(ctx, org)
	if err != nil || status >= 500 {
		if ok { // serve last-known through a brief outage
			return c.val, nil
		}
		if err == nil {
			err = errUpstream(status)
		}
		return upstream.Entitlement{}, err
	}
	if status >= 400 {
		return upstream.Entitlement{}, errUpstream(status)
	}
	a.mu.Lock()
	a.ent[org] = cached{at: time.Now(), val: ent}
	a.mu.Unlock()
	return ent, nil
}

func (a *Agent) invalidate(org string) {
	a.mu.Lock()
	delete(a.ent, org)
	a.mu.Unlock()
}

// ── Local HTTP API ───────────────────────────────────────────────────────────

func (a *Agent) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/can-use", a.canUse)
	mux.HandleFunc("POST /v1/report", a.report)
	mux.HandleFunc("POST /v1/authorize", a.authorize)
	mux.HandleFunc("GET /v1/features", a.features)
	mux.HandleFunc("GET /v1/subscription", a.subscription)
	mux.HandleFunc("GET /v1/credit", a.credit)
	mux.HandleFunc("POST /v1/provision/org", func(w http.ResponseWriter, r *http.Request) { a.proxy(w, r, "/api/v1/sdk/provisioning/orgs") })
	mux.HandleFunc("POST /v1/provision/trial", func(w http.ResponseWriter, r *http.Request) { a.proxy(w, r, "/api/v1/sdk/provisioning/subscriptions/trial") })
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]any{"status": "ok", "auth": a.up.AuthKind()}) })
	mux.HandleFunc("GET /readyz", a.readyz)
	return mux
}

type useReq struct {
	OrgID          string `json:"org_id"`
	Feature        string `json:"feature"`
	Units          int64  `json:"units"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (a *Agent) canUse(w http.ResponseWriter, r *http.Request) {
	var req useReq
	if !decode(w, r, &req) {
		return
	}
	ent, err := a.entitlement(r.Context(), req.OrgID)
	if err != nil {
		writeJSON(w, 200, map[string]any{"allow": a.failOpen, "remaining": nil, "reason": "billing_unreachable", "degraded": true, "fail_mode": failMode(a.failOpen)})
		return
	}
	f, ok := ent.Find(req.Feature)
	if !ok || !f.Enabled {
		writeJSON(w, 200, map[string]any{"allow": false, "remaining": 0, "reason": "feature_not_entitled", "plan": ent.PlanCode})
		return
	}
	// AUTHORITATIVE: reuse core's CanConsume — it already reflects the WHOLE funding
	// chain (base quota → addon → token → credit/wallet). We must NOT re-derive the
	// decision from base-quota `remaining`, or a feature that an add-on / token pool /
	// wallet would fund gets wrongly denied once the included quota runs out. `remaining`
	// is surfaced only as an informational base-quota hint; the binding gate is authorize.
	allow := f.CanConsume == nil || *f.CanConsume
	reason := ""
	if !allow {
		reason = "no_funding_available"
	}
	writeJSON(w, 200, map[string]any{
		"allow": allow, "remaining": remainingOf(f), "limit": limitOf(f),
		"funding_chain": f.FundingChain, "reset_at": f.ResetAt, "reason": reason, "plan": ent.PlanCode, "state": ent.SubState,
	})
}

// report records consumption after the fact. When batching is on, the event is
// buffered and flushed to core's batch endpoint (fewer core calls); the durable
// ledger is eventually-consistent. When off, it falls back to a sync consume.
func (a *Agent) report(w http.ResponseWriter, r *http.Request) {
	var req useReq
	if !decode(w, r, &req) {
		return
	}
	if req.Units <= 0 {
		req.Units = 1
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = newIdem()
	}
	if a.batcher != nil && a.batcher.Enabled() {
		a.batcher.Add(upstream.BatchEvent{
			OrgID: req.OrgID, FeatureCode: req.Feature, Units: req.Units,
			IdempotencyKey: req.IdempotencyKey, OccurredAt: time.Now().UTC(),
		})
		writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "queued": true, "idempotency_key": req.IdempotencyKey})
		return
	}
	a.syncConsume(w, r, req, false)
}

// authorize is the binding gate: always a synchronous, atomic core consume (never
// batched), so the allow/deny + money-funded path is authoritative with no race.
func (a *Agent) authorize(w http.ResponseWriter, r *http.Request) {
	var req useReq
	if !decode(w, r, &req) {
		return
	}
	if req.Units <= 0 {
		req.Units = 1
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = newIdem()
	}
	a.syncConsume(w, r, req, true)
}

func (a *Agent) syncConsume(w http.ResponseWriter, r *http.Request, req useReq, atomic bool) {
	res, status, err := a.up.Consume(r.Context(), req.OrgID, req.Feature, req.Units, req.IdempotencyKey)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "billing_unreachable"})
		return
	}
	a.invalidate(req.OrgID)
	out := map[string]any{
		"ok": res.Allowed, "allowed": res.Allowed, "reason": res.Reason,
		"units_consumed": res.UnitsConsumed, "funding_source": res.FundingSource, "payment_options": res.PaymentOptions,
	}
	if atomic && res.Allowed {
		if ent, e := a.entitlement(r.Context(), req.OrgID); e == nil {
			if f, ok := ent.Find(req.Feature); ok {
				out["remaining"] = remainingOf(f)
			}
		}
	}
	code := 200
	if !res.Allowed || status == http.StatusPaymentRequired {
		code = http.StatusPaymentRequired
	}
	writeJSON(w, code, out)
}

func (a *Agent) features(w http.ResponseWriter, r *http.Request) {
	ent, err := a.entitlement(r.Context(), r.URL.Query().Get("org_id"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "billing_unreachable"})
		return
	}
	writeJSON(w, 200, ent)
}

func (a *Agent) subscription(w http.ResponseWriter, r *http.Request) {
	ent, err := a.entitlement(r.Context(), r.URL.Query().Get("org_id"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "billing_unreachable"})
		return
	}
	writeJSON(w, 200, map[string]any{"plan_code": ent.PlanCode, "plan_name": ent.PlanName, "state": ent.SubState, "current_period": ent.CurrentPeriod})
}

func (a *Agent) credit(w http.ResponseWriter, r *http.Request) {
	ent, err := a.entitlement(r.Context(), r.URL.Query().Get("org_id"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "billing_unreachable"})
		return
	}
	writeJSON(w, 200, map[string]any{"balance_cents": ent.WalletBalance, "currency": ent.Currency})
}

func (a *Agent) proxy(w http.ResponseWriter, r *http.Request, path string) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	var out map[string]any
	status, err := a.up.Post(r.Context(), path, body, &out)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "billing_unreachable"})
		return
	}
	writeJSON(w, status, out)
}

func (a *Agent) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := a.up.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "core_unreachable"})
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ready"})
}

// ── helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return false
	}
	return true
}

func remainingOf(f upstream.Feature) *int64 {
	if f.Remaining != nil {
		return f.Remaining
	}
	return f.TotalQuota
}

func limitOf(f upstream.Feature) *int64 {
	if f.TotalQuota != nil {
		return f.TotalQuota
	}
	if f.PeriodQuota != nil {
		return f.PeriodQuota
	}
	return f.IncludedQuota
}

func failMode(open bool) string {
	if open {
		return "open"
	}
	return "closed"
}

func newIdem() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "agent_" + hex.EncodeToString(b)
}

type upErr int

func (e upErr) Error() string { return "upstream status " + http.StatusText(int(e)) }
func errUpstream(s int) error { return upErr(s) }
