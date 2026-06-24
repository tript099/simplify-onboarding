package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/simplify/onboarding/internal/catalog"
	"github.com/simplify/onboarding/internal/httpx"
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

// Entitlements returns the plan/entitlement snapshot for the current subject.
func (h *Handler) Entitlements(w http.ResponseWriter, r *http.Request) {
	subject := "anonymous"
	if data, _, err := h.sessions.FromRequest(r); err == nil {
		subject = data.UserID
	}
	ent, err := h.entitlements.Get(r.Context(), subject)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "entitlements_failed", "could not load entitlements")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ent)
}
