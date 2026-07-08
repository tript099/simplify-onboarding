package handler

import (
	"net/http"
	"strings"

	"github.com/simplify/onboarding/internal/catalog"
	"github.com/simplify/onboarding/internal/httpx"
	"go.uber.org/zap"
)

// productSub is the per-product subscription the frontend cards use.
type productSub struct {
	PlanCode         string `json:"planCode"`
	PlanName         string `json:"planName"`
	State            string `json:"state"` // ACTIVE | TRIALING | PAST_DUE | CANCELED | …
	BillingPeriod    string `json:"billingPeriod,omitempty"`
	PriceCents       int64  `json:"priceCents,omitempty"`
	Currency         string `json:"currency,omitempty"`
	CurrentPeriodEnd string `json:"currentPeriodEnd,omitempty"`
	CancelAtPeriodEnd bool  `json:"cancelAtPeriodEnd,omitempty"`
}

// Subscriptions returns the signed-in user's per-product subscription, keyed by the same
// product key the catalog uses (so cards can match). Requires a session; returns an empty
// map (200) for logged-out users or when the Account API isn't configured — cards then
// simply show the Purchase CTA.
// GET /auth/subscriptions
func (h *Handler) Subscriptions(w http.ResponseWriter, r *http.Request) {
	empty := map[string]any{"subscriptions": map[string]productSub{}}

	data, _, err := h.sessions.FromRequest(r)
	if err != nil || strings.TrimSpace(data.Email) == "" {
		httpx.WriteJSON(w, http.StatusOK, empty)
		return
	}
	if h.account == nil || !h.account.Enabled() {
		httpx.WriteJSON(w, http.StatusOK, empty)
		return
	}

	orgs, err := h.account.OrgsByEmail(r.Context(), data.Email)
	if err != nil {
		h.log.Warn("subscriptions: account api failed", zap.Error(err), zap.String("email", data.Email))
		httpx.WriteJSON(w, http.StatusOK, empty) // fail-soft: cards render without plans
		return
	}

	// Flatten across orgs → best subscription per product (prefer ACTIVE > TRIALING > …).
	best := map[string]productSub{}
	rank := map[string]int{"ACTIVE": 4, "TRIALING": 3, "PAST_DUE": 2, "CANCELED": 1}
	for _, o := range orgs {
		for _, s := range o.Subscriptions {
			key := catalog.NormalizeKey(s.ProductSlug)
			cand := productSub{
				PlanCode: s.PlanCode, PlanName: s.PlanName, State: strings.ToUpper(s.State),
				BillingPeriod: s.BillingPeriod, PriceCents: s.PriceCents, Currency: s.Currency,
				CurrentPeriodEnd: s.CurrentPeriodEnd, CancelAtPeriodEnd: s.CancelAtPeriodEnd,
			}
			if cur, ok := best[key]; !ok || rank[cand.State] > rank[cur.State] {
				best[key] = cand
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"subscriptions": best})
}
