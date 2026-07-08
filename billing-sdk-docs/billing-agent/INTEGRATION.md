# Billing Agent — Integration Guide

> The product-side **billing PDP** (Policy Decision Point) — the *Cerbos of quota*.
> Your product/BFF asks two simple questions; the agent hides every billing detail.

This is the complete, hands-on guide: what it is, how to run it, every endpoint with
request/response examples, BFF wiring in three languages, caching + consumption
write-back, the two auth modes (API key / Zitadel), resilience, deployment, and a full
config + troubleshooting reference.

---

## Table of contents
1. [Mental model](#1-mental-model)
2. [Architecture](#2-architecture)
3. [Quickstart (5 minutes)](#3-quickstart)
4. [The local API — every endpoint](#4-the-local-api)
5. [BFF integration patterns](#5-bff-integration-patterns)
6. [Reads & caching](#6-reads--caching)
7. [Consumption write-back (batching)](#7-consumption-write-back)
8. [Authentication: API key vs Zitadel](#8-authentication)
9. [Resilience: fail modes & stale-on-error](#9-resilience)
10. [Deployment](#10-deployment)
11. [Config reference](#11-config-reference)
12. [Troubleshooting](#12-troubleshooting)

---

## 1. Mental model

Authorization (Cerbos) answers *"can this principal do this action?"* → ALLOW/DENY.
The billing agent answers the **billing** version with the same ergonomics:

```
CanUse(org, feature)        → can this org use this feature, and how much is left?
Report(org, feature, n)     → this org just used n units (after the fact)
Authorize(org, feature, n)  → check AND reserve atomically (no race)  ← strict features
```

Your product never implements: entitlement resolution across plan + add-ons + token
pools, overage policy, daily/period reset windows, wallet/Simplify-Credit funding,
caching, usage batching, or talking to core. It just asks the agent.

**Two questions, on the hot path:**

```
allow := agent.CanUse(org, "ai_interview")     // before serving
if !allow.allow:  return 402 + allow.payment_options
... run the feature ...
agent.Report(org, "ai_interview", unitsUsed)   // after — or use Authorize to fuse both
```

---

## 2. Architecture

```
┌─────────────────────────────────────┐         ┌──────────────────────────┐
│  Your product pod / host            │         │   SimplifyBilling core    │
│                                     │  Bearer │   /api/v1/sdk/*           │
│  ┌────────┐   localhost:8099  ┌─────┴────┐    │   (API-key or Zitadel)    │
│  │  BFF   │ ───────────────►  │ billing- │ ──►│   entitlements / metering │
│  │ / app  │  can-use/report   │  agent   │    │   provisioning            │
│  └────────┘                   └─────┬────┘    └──────────────────────────┘
│                                cache│ + batch                              
└─────────────────────────────────────┘                                      
```

- **Sidecar container** (recommended): runs next to your product. Your BFF calls it on
  `localhost:8099` with a tiny **unauthenticated** local API (localhost-only). The agent
  holds the product credential, caches reads, batches usage, and talks to core. Works
  from **any language**.
- **Go library** (`libs/billing-sdk-go`): Go products can import the same logic directly.

Internally the agent is modular (`internal/`):

| Package | Responsibility |
|---|---|
| `config`   | env loading |
| `auth`     | pluggable Authenticator — static API key **or** Zitadel M2M tokens |
| `upstream` | typed client for core `/api/v1/sdk/*` |
| `batch`    | report buffering + flush (consumption write-back) |
| `pdp`      | cache (stale-on-error) + can-use/report/authorize + the HTTP API |

---

## 3. Quickstart

### 3.1 Get a product API key
Admin portal → **Integration** → *Rotate API key*. You get a `bk_…` key **once** — store
it as a secret. (Or use Zitadel M2M — see §8.)

### 3.2 Run the sidecar
`docker-compose.yml`:
```yaml
services:
  billing-agent:
    image: ghcr.io/simplify/billing-agent:latest      # or build ./billing-agent
    environment:
      BILLING_BASE_URL: https://core.simplify.example   # SimplifyBilling core
      BILLING_API_KEY:  ${BILLING_API_KEY}              # bk_… (from a secret)
      AGENT_LISTEN:     ":8099"
      AGENT_FAIL_MODE:  closed                          # deny if core unreachable
      AGENT_CACHE_TTL:  45s
      AGENT_FLUSH_INTERVAL: 2s
      AGENT_FLUSH_SIZE: "50"
    ports: ["8099:8099"]                                # localhost-only in real deploys
```

### 3.3 Smoke test
```bash
curl localhost:8099/healthz
# {"auth":"api_key","status":"ok"}

curl localhost:8099/readyz
# {"status":"ready"}     (200; 503 if core unreachable)

curl -s -X POST localhost:8099/v1/can-use \
  -d '{"org_id":"<ORG_UUID>","feature":"ai_interview"}'
# {"allow":true,"remaining":16,"limit":16,"reset_at":"...","reason":"","plan":"pro","state":"ACTIVE"}
```

That's it — your product can now gate on billing.

---

## 4. The local API

All POST bodies are JSON. `org_id` is the SimplifyBilling org UUID; `feature` is the
feature code from your catalog.

### `POST /v1/can-use` — the gate (read-only, advisory)
"Can org use feature, and how much is left?" Served from cache; **does not** decrement.

Request:
```json
{ "org_id": "34780f66-…", "feature": "ai_interview" }
```
Response (allowed):
```json
{
  "allow": true,
  "remaining": 16,
  "limit": 16,
  "funding_chain": ["SUBSCRIPTION_QUOTA", "ADDON_GRANTS", "TOKEN_POOL", "WALLET"],
  "reset_at": "2026-07-01T00:00:00Z",
  "reason": "",
  "plan": "pro",
  "state": "ACTIVE"
}
```
`allow` reflects core's **whole funding chain** (`can_consume`) — it stays `true` while *any*
source in the chain can fund (add-on / token pool / Simplify-Credit wallet), even after the
base `remaining` hits 0. `remaining` is only the base-quota hint; the binding decision is
`/v1/authorize`.

Response (denied — no source in the chain can fund):
```json
{ "allow": false, "remaining": 0, "limit": 16, "funding_chain": ["SUBSCRIPTION_QUOTA"], "reason": "no_funding_available", "plan": "pro", "state": "ACTIVE" }
```
Response (denied — feature not on plan):
```json
{ "allow": false, "remaining": 0, "reason": "feature_not_entitled", "plan": "pro" }
```
Response (core unreachable, fail-closed):
```json
{ "allow": false, "remaining": null, "reason": "billing_unreachable", "degraded": true, "fail_mode": "closed" }
```

`reason` values: `""` (ok) · `no_funding_available` · `feature_not_entitled` · `billing_unreachable`.

### `POST /v1/report` — record usage (after the fact, batched)
"Org used N units." Buffered and flushed to core in batches (see §7). Returns immediately.

Request:
```json
{ "org_id": "34780f66-…", "feature": "ai_interview", "units": 3, "idempotency_key": "job-abc-123" }
```
Response:
```json
{ "ok": true, "queued": true, "idempotency_key": "job-abc-123" }   // HTTP 202
```
- `units` defaults to 1. `idempotency_key` is auto-generated if omitted — **pass your own**
  (e.g. the job/request id) to make retries safe.

### `POST /v1/authorize` — atomic check + reserve (strict features)
Checks **and** decrements in one atomic core call. No race, no overspend. Use for
expensive/paid features where a stale `can-use` could over-allow.

Request:
```json
{ "org_id": "34780f66-…", "feature": "ai_interview", "units": 1, "idempotency_key": "req-789" }
```
Response (allowed):
```json
{ "ok": true, "allowed": true, "remaining": 15, "units_consumed": 1, "funding_source": "SUBSCRIPTION_QUOTA", "payment_options": null }
```
Response (denied — `HTTP 402`):
```json
{ "ok": false, "allowed": false, "reason": "quota_exhausted", "units_consumed": 0, "funding_source": "", "payment_options": [...] }
```
`funding_source`: `SUBSCRIPTION_QUOTA` · `TOKEN_POOL` · `WALLET` (Simplify-Credit) ·
`POSTPAID`. When it's `WALLET`/`POSTPAID`, the call moved money — always synchronous.

### `GET /v1/features?org_id=…` — full entitlement snapshot
All features + quota/remaining + plan, for dashboards.
```json
{ "org_id":"…","plan_code":"pro","subscription_state":"ACTIVE",
  "wallet_balance_cents":97640,"currency":"SBC",
  "features":[{"code":"ai_interview","enabled":true,"remaining":16,"total_effective_quota":16,"reset_at":"…"}] }
```

### `GET /v1/subscription?org_id=…`
```json
{ "plan_code":"pro","plan_name":"Pro Monthly","state":"ACTIVE",
  "current_period":{"start":"…","end":"…"} }
```

### `GET /v1/credit?org_id=…`
```json
{ "balance_cents": 97640, "currency": "SBC" }
```

### `POST /v1/provision/org` — ensure an org exists (idempotent)
Call on first product-side login to map your `external_id` → a SimplifyBilling org.
All four fields are required (`external_id`, `name`, `owner_user_id`, `owner_email`):
```json
{ "external_id": "acme-corp", "name": "Acme Corp", "owner_user_id": "your-product-user-id", "owner_email": "ceo@acme.com" }
```

### `POST /v1/provision/trial` — start a trial
```json
{ "org_id": "…", "plan_code": "pro-trial" }   // plan_code optional if app default set
```

### `GET /healthz` · `GET /readyz`
`healthz` → 200 always (process up, shows auth mode). `readyz` → 200 if core reachable,
503 otherwise (use for k8s readiness so traffic isn't sent before billing is reachable —
or NOT, if you want the product to run on cache during a billing blip; your call).

---

## 5. BFF integration patterns

### Go
```go
type Gate struct{ base string; hc *http.Client }

func (g Gate) CanUse(ctx context.Context, org, feature string) (bool, error) {
    body, _ := json.Marshal(map[string]any{"org_id": org, "feature": feature})
    req, _ := http.NewRequestWithContext(ctx, "POST", g.base+"/v1/can-use", bytes.NewReader(body))
    resp, err := g.hc.Do(req)
    if err != nil { return false, err }
    defer resp.Body.Close()
    var r struct{ Allow bool `json:"allow"` }
    json.NewDecoder(resp.Body).Decode(&r)
    return r.Allow, nil
}

// handler
if ok, _ := gate.CanUse(ctx, orgID, "ai_interview"); !ok {
    http.Error(w, "quota exceeded", http.StatusPaymentRequired); return
}
runInterview()
go gate.Report(ctx, orgID, "ai_interview", 1)   // fire-and-forget (batched server-side)
```

### Node / TypeScript
```ts
const AGENT = "http://localhost:8099";
async function canUse(org: string, feature: string) {
  const r = await fetch(`${AGENT}/v1/can-use`, {
    method: "POST", headers: { "content-type": "application/json" },
    body: JSON.stringify({ org_id: org, feature }),
  });
  return r.json() as Promise<{ allow: boolean; remaining: number; reason: string }>;
}
async function report(org: string, feature: string, units = 1, idem?: string) {
  await fetch(`${AGENT}/v1/report`, {
    method: "POST", headers: { "content-type": "application/json" },
    body: JSON.stringify({ org_id: org, feature, units, idempotency_key: idem }),
  });
}

// express middleware
app.post("/interview", async (req, res) => {
  const { allow, reason } = await canUse(req.org, "ai_interview");
  if (!allow) return res.status(402).json({ error: reason });
  const result = await runInterview(req.body);
  report(req.org, "ai_interview", 1, req.id);   // batched
  res.json(result);
});
```

### Python
```python
import httpx
AGENT = "http://localhost:8099"

def can_use(org, feature):
    return httpx.post(f"{AGENT}/v1/can-use", json={"org_id": org, "feature": feature}).json()

def report(org, feature, units=1, idem=None):
    httpx.post(f"{AGENT}/v1/report", json={"org_id": org, "feature": feature, "units": units, "idempotency_key": idem})

# FastAPI
@app.post("/interview")
def interview(req: Req):
    d = can_use(req.org, "ai_interview")
    if not d["allow"]:
        raise HTTPException(402, d["reason"])
    out = run_interview(req)
    report(req.org, "ai_interview", 1, req.id)   # batched
    return out
```

### Choosing can-use+report vs authorize
| Feature kind | Pattern | Why |
|---|---|---|
| Cheap / soft (page views, soft limits) | `can-use` then `report` | simplest; tiny over-allow window is fine |
| Expensive / paid (LLM calls, paid actions) | `authorize` | atomic; can never over-allow / overspend |

---

## 6. Reads & caching

The agent caches the **entitlement structure** (plan, limits, feature list, sub state,
credit) per org. So `can-use` / `features` / `subscription` / `credit` are served locally
without hitting core on a cache hit.

- **TTL**: `AGENT_CACHE_TTL` (default 45s) — a freshness backstop.
- **Invalidation**: on every `report`/`authorize` for an org, that org's cache is busted
  (read-your-writes). In production you also wire **event invalidation** (core emits
  `subscription.*` / `addon.purchased` → the agent busts the org) so structure refreshes
  *only when it actually changes*, not on a timer. See `CACHING.md` for the full design.
- **No dirty read**: `can-use` is **advisory** (may be slightly stale); the **binding**
  decision is `authorize` (atomic at core). Gate strict features on `authorize`.

---

## 7. Consumption write-back

`report` is **batched**; `authorize` is **synchronous** (binding gate, money-safe).

```
report  → buffer ──(every AGENT_FLUSH_SIZE events OR AGENT_FLUSH_INTERVAL OR on shutdown)──► core /sdk/metering/consume/batch
authorize → core /sdk/metering/consume   (immediately, atomic)
```

- **At-least-once**: every event carries an idempotency key; a failed flush is **re-queued**
  and core **dedups**. No double counting, no lost usage.
- **Graceful shutdown**: on SIGTERM the agent flushes the buffer before exiting — no usage
  lost on deploy/restart.
- **Money is never batched**: when consumption funds from wallet/postpaid, it goes through
  synchronous `authorize`.
- **Disable batching**: set `AGENT_FLUSH_SIZE=0` or `AGENT_FLUSH_INTERVAL=0` → `report`
  becomes synchronous.

Tuning: higher `FLUSH_SIZE`/`INTERVAL` = fewer core calls, larger loss window on a hard
crash (in-memory buffer). For billing-grade durability at scale, graduate to a Redis-stream
buffer (see `CACHING.md` Phase C).

---

## 8. Authentication

The agent's upstream auth is **pluggable** — same code, switch by config.

### 8.1 Static API key (default)
```
BILLING_API_KEY=bk_…
```
- Minted/rotated in admin → Integration. Stored hashed (SHA-256) at core; sent as
  `Authorization: Bearer bk_…`.
- Simple; long-lived. Rotate via the admin UI (old key works during a grace window).

### 8.2 Zitadel M2M (enterprise)
Each tenant gets a Zitadel **service user**; the agent fetches short-lived tokens via
client-credentials and caches them (refresh ~60s before expiry).
```
ZITADEL_TOKEN_URL=https://<instance>.zitadel.cloud/oauth/v2/token
ZITADEL_CLIENT_ID=…
ZITADEL_CLIENT_SECRET=…
ZITADEL_SCOPE=openid urn:zitadel:iam:org:project:id:<projectID>:aud
```
When set, the agent uses Zitadel instead of `BILLING_API_KEY`. `healthz` reports
`"auth":"zitadel_m2m"`.

> **Implemented & tested.** Core validates the JWT on `/sdk` (`httpx.RequireProductAuth`:
> JWKS RS256 + issuer + expiry), mapping the token `sub` → `app_id` via the
> `apps.zitadel_subject` column — hybrid with the API key (both work simultaneously). To
> onboard a tenant onto Zitadel: create a Zitadel machine user, set its client secret, and
> set `apps.zitadel_subject` to that machine-user id. See `PLAN.md` → *Zitadel M2M*.

| | API key | Zitadel M2M |
|---|---|---|
| Secret lifetime | long-lived | short-lived JWT (auto-refresh) |
| Rotation/revocation | manual (admin) | central, instant |
| Scopes/roles/audit | none | project roles + audit |

---

## 9. Resilience

When core is unreachable the agent degrades predictably:

| Situation | Behaviour |
|---|---|
| Cache **hit** (within TTL) | served locally — no core call |
| Cache **expired** + core down, but a **prior snapshot exists** | **stale-on-error**: serve last-known (`degraded` not set on the data; quota not extended) |
| **No** cached snapshot + core down, `AGENT_FAIL_MODE=open` | `can-use` → `allow:true, degraded:true` (let non-critical features through) |
| **No** cached snapshot + core down, `AGENT_FAIL_MODE=closed` | `can-use` → `allow:false, degraded:true` (protect paid features) |
| `authorize` + core down | `502` (never silently allow a binding paid decision) |
| Bad/invalid API key | core `401` → treated as error → fail-mode applies |

Choose `AGENT_FAIL_MODE` per deployment: `closed` (default) protects revenue; `open`
favours availability for non-critical features. (Per-feature fail policy is on the roadmap.)

---

## 10. Deployment

### Kubernetes sidecar
```yaml
spec:
  containers:
    - name: app
      # ... your product ...
      env: [{ name: BILLING_AGENT_URL, value: "http://localhost:8099" }]
    - name: billing-agent
      image: ghcr.io/simplify/billing-agent:latest
      env:
        - { name: BILLING_BASE_URL, value: "https://core.simplify.example" }
        - { name: BILLING_API_KEY, valueFrom: { secretKeyRef: { name: billing, key: api-key } } }
        - { name: AGENT_FAIL_MODE, value: "closed" }
      ports: [{ containerPort: 8099 }]
      readinessProbe: { httpGet: { path: /readyz, port: 8099 } }
      livenessProbe:  { httpGet: { path: /healthz, port: 8099 } }
```
- The image is **distroless, non-root**. One agent **per pod** (sidecar) keeps the local
  call on loopback (no network auth needed) and isolates cache/batch per replica.
- **Scaling**: each replica caches independently (fine). For shared cache/counter across
  many replicas, graduate to the Redis tier (`CACHING.md` Phase B/C).
- **Graceful drains**: ensure `terminationGracePeriodSeconds ≥ 15` so the shutdown flush
  completes.

---

## 11. Config reference

| Env | Default | Meaning |
|---|---|---|
| `BILLING_BASE_URL` | `http://core:8081` | SimplifyBilling core base URL |
| `BILLING_API_KEY` | — | product API key `bk_…` (required unless Zitadel) |
| `AGENT_LISTEN` | `:8099` | local API listen addr |
| `AGENT_CACHE_TTL` | `45s` | entitlement cache freshness backstop |
| `AGENT_FAIL_MODE` | `closed` | `closed`=deny / `open`=allow when core unreachable & uncached |
| `AGENT_FLUSH_INTERVAL` | `2s` | report batch flush interval (0 disables batching) |
| `AGENT_FLUSH_SIZE` | `50` | report batch flush size (0 disables batching) |
| `ZITADEL_TOKEN_URL` | — | enable Zitadel M2M auth |
| `ZITADEL_CLIENT_ID` / `_SECRET` / `_SCOPE` | — | Zitadel client-credentials |

---

## 12. Troubleshooting

| Symptom | Cause / fix |
|---|---|
| `healthz` ok but every `can-use` is `degraded` | core unreachable — check `BILLING_BASE_URL`, network, `readyz` |
| `401` from core / all calls degraded | wrong/expired `BILLING_API_KEY`; rotate in admin → Integration |
| `feature_not_entitled` for a real feature | feature not on the org's plan, or wrong `feature` code (must match the catalog code) |
| `report` returns 202 but quota not changing | batching — usage applies on the next flush (`AGENT_FLUSH_INTERVAL`); lower it to see sooner, or use `authorize` |
| Usage lost on deploy | ensure SIGTERM reaches the agent (`init: true` / tini) and grace period ≥ 15s |
| `authorize` returns `402` with `payment_options` | quota exhausted and no free funding — surface the payment options / upsell |
| Quota looks stale right after a change | cache TTL — wire event invalidation (`subscription.*` webhook → bust org) for instant refresh |

---

## See also
- `PLAN.md` — design, phases, endpoint mapping, Zitadel.
- `CACHING.md` — caching & consistency (no-dirty-read), consumption write-back internals.
- **Product webhook & user onboarding** — `docs/features/product-onboarding/WEBHOOK_ONBOARDING.md`.
