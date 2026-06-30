package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/simplify/onboarding/internal/catalog"
	"github.com/simplify/onboarding/internal/httpx"
	"github.com/simplify/onboarding/internal/notify"
	"github.com/simplify/onboarding/internal/scheduler"
	"github.com/simplify/onboarding/internal/store"
	"go.uber.org/zap"
)

// Clients returns the public product registry (drives the homepage + brand panel).
func (h *Handler) Clients(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, h.catalog.List())
}

// ProductMotion returns the motion + CTA descriptor for one product.
func (h *Handler) ProductMotion(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	p, ok := h.catalog.Get(key)
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "unknown product")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"key":              p.Key,
		"name":             p.Name,
		"intent":           p.Intent,
		"allowedUserTypes": p.AllowedTypes,
		"asksUserType":     p.AsksUserType,
		"enterpriseOnly":   p.EnterpriseOnly,
		"trialScope":       p.TrialScope,
		"dataResidency":    p.DataResidency,
		"primaryCta":       catalog.PrimaryCTA(p, false),
	})
}

// OnbSession mints (or returns) the anonymous visitor identity.
func (h *Handler) OnbSession(w http.ResponseWriter, r *http.Request) {
	id, err := h.visitors.EnsureCookie(w, r)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not start session")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"visitorId": id})
}

type eventRequest struct {
	Event  string `json:"event"`
	Intent string `json:"intent"`
}

// OnbEvent records a funnel event for the current visitor.
func (h *Handler) OnbEvent(w http.ResponseWriter, r *http.Request) {
	var req eventRequest
	if err := httpx.DecodeJSON(r, &req); err != nil || req.Event == "" {
		bad(w, "event is required")
		return
	}
	id, err := h.visitors.EnsureCookie(w, r)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not record event")
		return
	}
	if err := h.visitors.Record(r.Context(), id, req.Event, req.Intent); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not record event")
		return
	}
	// Durable analytics trail (best-effort — Redis already holds resumable state).
	if h.db != nil {
		userID := "" // universal identity (Zitadel sub), not any product's id
		if data, _, err := h.sessions.FromRequest(r); err == nil {
			userID = data.ZitadelSub
		}
		if err := h.db.InsertEvent(r.Context(), id, userID, req.Event, req.Intent, nil); err != nil {
			h.log.Warn("event persist failed", zap.Error(err))
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// OnbState returns the visitor's current funnel position so the SPA can resume.
func (h *Handler) OnbState(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("visitor_id")
	if err != nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"stage": "anonymous"})
		return
	}
	st, err := h.visitors.Get(r.Context(), c.Value)
	if err != nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"stage": "anonymous"})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, st)
}

// SubmitDemo captures a sales-led request (demo / POC / contact). It persists the
// request to Redis (a capped list) and logs it; a CRM/email hook plugs in here.
func (h *Handler) SubmitDemo(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if err := httpx.DecodeJSON(r, &req); err != nil {
		bad(w, "invalid request body")
		return
	}
	if t, _ := req["type"].(string); t == "" {
		req["type"] = "demo"
	}
	reqType, _ := req["type"].(string)
	h.log.Info("demo request received",
		zap.Any("type", req["type"]),
		zap.Any("product", req["product"]),
		zap.Any("email", req["email"]),
		zap.Any("company", req["company"]),
	)

	// Primary store: the onboarding DB (durable sales pipeline). Redis is a fallback
	// when the DB isn't configured (offline dev).
	id := ""
	if h.db != nil {
		newID, err := h.db.InsertDemoRequest(r.Context(), store.DemoRequest{
			Type:          reqType,
			ProductKey:    str(req["product"]),
			FirstName:     str(req["firstName"]),
			LastName:      str(req["lastName"]),
			Email:         str(req["email"]),
			Phone:         str(req["phone"]),
			JobTitle:      str(req["jobTitle"]),
			Company:       str(req["company"]),
			CompanySize:   str(req["companySize"]),
			Industry:      str(req["industry"]),
			Country:       str(req["country"]),
			UseCase:       str(req["useCase"]),
			Timeline:      str(req["timeline"]),
			Budget:        str(req["budget"]),
			Notes:         str(req["notes"]),
			Message:       str(req["message"]),
			PreferredDate: str(req["preferredDate"]),
			TimeSlot:      str(req["timeSlot"]),
			Timezone:      str(req["timezone"]),
			Consent:       boolVal(req["consent"]),
			Payload:       req,
		})
		if err != nil {
			h.log.Error("demo request: persist failed", zap.Error(err))
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not submit your request")
			return
		}
		id = newID
	} else {
		id = uuid.NewString()
		req["requestId"] = id
		req["receivedAt"] = time.Now().UTC().Format(time.RFC3339)
		if h.rdb != nil {
			if raw, err := json.Marshal(req); err == nil {
				ctx := r.Context()
				_ = h.rdb.LPush(ctx, "demo_requests", raw).Err()
				_ = h.rdb.LTrim(ctx, "demo_requests", 0, 999).Err()
			}
		}
	}

	// Fire the emails (requester confirmation + team notification) asynchronously —
	// the request is already saved, so email never blocks or fails the submission.
	if h.notify != nil && h.notify.Enabled() && id != "" {
		nreq := notify.Request{
			ID:            id,
			Type:          reqType,
			ProductKey:    str(req["product"]),
			ProductName:   h.productName(str(req["product"])),
			FirstName:     str(req["firstName"]),
			LastName:      str(req["lastName"]),
			Email:         str(req["email"]),
			Phone:         str(req["phone"]),
			JobTitle:      str(req["jobTitle"]),
			Company:       str(req["company"]),
			CompanySize:   str(req["companySize"]),
			Industry:      str(req["industry"]),
			Country:       str(req["country"]),
			UseCase:       str(req["useCase"]),
			Timeline:      str(req["timeline"]),
			Budget:        str(req["budget"]),
			Message:       str(req["message"]),
			PreferredDate: str(req["preferredDate"]),
			TimeSlot:      str(req["timeSlot"]),
			Timezone:      str(req["timezone"]),
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer cancel()

			// If the requester picked a date/time and scheduling is available, book the
			// meeting now and send them the INVITE (with .ics). Otherwise just confirm.
			cid, nid := "", ""
			if meeting, startAt, ok := h.autoSchedule(ctx, id, nreq); ok {
				// Scheduled → both the requester and the team get a branded email with
				// the join link + .ics (one organized email each, no raw invites).
				cid = h.notify.SendInvite(ctx, nreq, meeting.URL, whenLine(startAt), startAt, 0)
				nid = h.notify.SendTeamNotify(ctx, nreq, meeting.URL, whenLine(startAt), startAt, 0)
			} else {
				cid = h.notify.SendConfirmation(ctx, nreq)
				nid = h.notify.SendTeamNotify(ctx, nreq, "", "", time.Time{}, 0)
			}
			if h.db != nil && (cid != "" || nid != "") {
				if err := h.db.UpdateDemoEmails(ctx, id, cid, nid); err != nil {
					h.log.Warn("demo request: store email ids failed", zap.Error(err))
				}
			}
		}()
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "requestId": id})
}

// autoSchedule books a meeting for a demo/POC that has a preferred date — turning the
// "preferred time" into an actual Google Meet / Teams meeting. Returns false when not
// applicable (no date, contact-type, scheduler off) or on failure (caller falls back
// to a plain confirmation). The team are the scheduler attendees; the requester gets
// the branded invite + .ics separately.
func (h *Handler) autoSchedule(ctx context.Context, id string, nreq notify.Request) (*scheduler.Meeting, time.Time, bool) {
	if h.scheduler == nil || !h.scheduler.Enabled() {
		return nil, time.Time{}, false
	}
	if nreq.Type == "contact" || nreq.PreferredDate == "" {
		return nil, time.Time{}, false
	}
	startAt, err := deriveStartTime(nreq.PreferredDate, nreq.TimeSlot, nreq.Timezone)
	if err != nil {
		return nil, time.Time{}, false
	}
	// Create the meeting with a no-op organizer attendee so the scheduler doesn't email
	// anyone its raw (unbranded) invite. The requester + team get OUR branded emails,
	// each carrying the join link + an .ics to add it to their calendar.
	attendees := []string{h.cfg.MailforgeFromEmail}
	summary := fmt.Sprintf("Simplify %s — %s", titleFor(nreq.Type), orStr(nreq.ProductName, "Simplify"))
	desc := fmt.Sprintf("%s for %s. Use-case discussion with the Simplify team.", titleFor(nreq.Type), orStr(nreq.Company, nreq.Email))
	meeting, err := h.scheduler.Schedule(ctx, attendees, startAt, 0, summary, desc)
	if err != nil {
		h.log.Warn("demo request: auto-schedule failed — sending plain confirmation", zap.Error(err), zap.String("request_id", id))
		return nil, time.Time{}, false
	}
	if h.db != nil {
		if err := h.db.UpdateDemoScheduled(ctx, id, meeting.URL, startAt, ""); err != nil {
			h.log.Warn("demo request: store schedule failed", zap.Error(err))
		}
	}
	return meeting, startAt, true
}

type scheduleRequest struct {
	StartTime string `json:"start_time"` // RFC3339 or "2006-01-02T15:04:05"
	Duration  int    `json:"duration"`   // minutes (optional)
}

// ScheduleDemo books a meeting for a demo/POC request via the external scheduler,
// stores the join URL + time, and emails the invite. Internal endpoint (a rep/admin
// confirms a slot) — guarded by the internal secret when one is configured.
func (h *Handler) ScheduleDemo(w http.ResponseWriter, r *http.Request) {
	if h.cfg.InternalSecret != "" && r.Header.Get("X-Internal-Secret") != h.cfg.InternalSecret {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid internal secret")
		return
	}
	if h.scheduler == nil || !h.scheduler.Enabled() {
		httpx.WriteError(w, http.StatusServiceUnavailable, "scheduler_unavailable", "scheduler is not configured")
		return
	}
	if h.db == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "db_unavailable", "database not configured")
		return
	}

	id := chi.URLParam(r, "id")
	var req scheduleRequest
	if err := httpx.DecodeJSON(r, &req); err != nil || req.StartTime == "" {
		bad(w, "start_time is required")
		return
	}
	startAt, err := parseStartTime(req.StartTime)
	if err != nil {
		bad(w, "invalid start_time (use RFC3339, e.g. 2026-06-28T15:00:00Z)")
		return
	}

	dr, err := h.db.GetDemoRequest(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "request not found")
		return
	}
	if dr.Email == "" {
		bad(w, "request has no requester email")
		return
	}

	productName := h.productName(dr.ProductKey)
	// No-op organizer attendee so the scheduler doesn't send its raw invite — the
	// requester gets our branded invite + .ics below.
	attendees := []string{h.cfg.MailforgeFromEmail}
	summary := fmt.Sprintf("Simplify %s — %s", titleFor(dr.Type), orStr(productName, "Simplify"))
	desc := fmt.Sprintf("%s for %s. Use case discussion with the Simplify team.", titleFor(dr.Type), orStr(dr.Company, dr.Email))

	meeting, err := h.scheduler.Schedule(r.Context(), attendees, startAt, req.Duration, summary, desc)
	if err != nil {
		h.log.Error("schedule demo: scheduler failed", zap.Error(err), zap.String("request_id", id))
		httpx.WriteError(w, http.StatusBadGateway, "schedule_failed", "could not schedule the meeting")
		return
	}

	inviteID := ""
	if h.notify != nil {
		inviteID = h.notify.SendInvite(r.Context(), notify.Request{
			ID: id, Type: dr.Type, ProductKey: dr.ProductKey, ProductName: productName,
			FirstName: dr.FirstName, LastName: dr.LastName, Email: dr.Email,
		}, meeting.URL, whenLine(startAt), startAt, req.Duration)
	}
	if err := h.db.UpdateDemoScheduled(r.Context(), id, meeting.URL, startAt, inviteID); err != nil {
		h.log.Warn("schedule demo: store update failed", zap.Error(err))
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"ok": true, "meeting_url": meeting.URL, "scheduled_at": startAt.UTC().Format(time.RFC3339),
	})
}

// parseStartTime accepts RFC3339 (with/without zone) or a naive ISO timestamp (UTC).
func parseStartTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable time")
}

func titleFor(t string) string {
	switch t {
	case "poc":
		return "POC"
	case "contact":
		return "call"
	default:
		return "Demo"
	}
}

func orStr(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

// deriveStartTime turns a preferred date + loose time slot ("Morning (9–12)") +
// timezone into a concrete start time. Falls back to UTC if the zone can't be loaded.
func deriveStartTime(dateStr, slot, tz string) (time.Time, error) {
	d, err := time.Parse("2006-01-02", strings.TrimSpace(dateStr))
	if err != nil {
		return time.Time{}, err
	}
	loc := time.UTC
	if tz != "" {
		if l, err := time.LoadLocation(strings.TrimSpace(tz)); err == nil {
			loc = l
		}
	}
	hour := 10 // "Any time" / unknown → mid-morning
	switch {
	case strings.Contains(slot, "Morning"):
		hour = 9
	case strings.Contains(slot, "Afternoon"):
		hour = 13
	case strings.Contains(slot, "Evening"):
		hour = 16
	}
	return time.Date(d.Year(), d.Month(), d.Day(), hour, 0, 0, 0, loc), nil
}

// whenLine formats a meeting time for emails (in its own timezone).
func whenLine(t time.Time) string {
	return t.Format("Mon, 2 Jan 2006 · 15:04 MST")
}

// productName resolves a product key to its display name via the catalog.
func (h *Handler) productName(key string) string {
	if key == "" {
		return ""
	}
	if p, ok := h.catalog.Get(key); ok {
		return p.Name
	}
	return ""
}

// Entitlements returns the plan/entitlement snapshot for the current subject.
func (h *Handler) Entitlements(w http.ResponseWriter, r *http.Request) {
	subject := "anonymous"
	if data, _, err := h.sessions.FromRequest(r); err == nil {
		subject = data.ZitadelSub // universal identity
	}
	ent, err := h.entitlements.Get(r.Context(), subject)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "entitlements_failed", "could not load entitlements")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ent)
}

// str coerces a decoded-JSON value to a trimmed string ("" for non-strings/nil).
func str(v any) string {
	s, _ := v.(string)
	return s
}

// boolVal coerces a decoded-JSON value to a bool (handles bool and "true").
func boolVal(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true"
	default:
		return false
	}
}
