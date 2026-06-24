// Package entitlements reads plan/entitlement state from SimplifyCore. This
// service never owns billing — it only reads, to decide value-wall vs trial vs
// locked and to render the right CTA. A stub returns a free trial until Core ships.
package entitlements

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// ProductEntitlement is the per-product plan state for an org/user.
type ProductEntitlement struct {
	Key         string `json:"key"`
	Plan        string `json:"plan"`        // free_trial | paid | locked
	TrialStatus string `json:"trialStatus"` // active | expired | none
	Credits     int    `json:"credits"`
}

// Entitlements is the full snapshot for a subject.
type Entitlements struct {
	Org      string               `json:"org,omitempty"`
	Products []ProductEntitlement `json:"products"`
}

// Client fetches entitlements from SimplifyCore (or returns a stub).
type Client struct {
	baseURL string
	token   string
	stub    bool
	keys    []string
	http    *http.Client
	log     *zap.Logger
}

// New builds the client. When stub is true (or no baseURL), Get returns a
// free-trial entitlement for every product so the UI works before Core exists.
func New(baseURL, token string, stub bool, productKeys []string, log *zap.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		stub:    stub || baseURL == "",
		keys:    productKeys,
		http:    &http.Client{Timeout: 8 * time.Second},
		log:     log,
	}
}

// Get returns the entitlement snapshot for a subject (user or org id).
func (c *Client) Get(ctx context.Context, subject string) (Entitlements, error) {
	if c.stub {
		return c.freeTrial(), nil
	}
	url := fmt.Sprintf("%s/entitlements?subject=%s", c.baseURL, subject)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Entitlements{}, fmt.Errorf("build entitlements request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		// Fail open to a free trial rather than blocking onboarding on Core being down.
		c.log.Warn("entitlements fetch failed, defaulting to free trial", zap.Error(err))
		return c.freeTrial(), nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		c.log.Warn("entitlements non-2xx, defaulting to free trial", zap.Int("status", resp.StatusCode))
		return c.freeTrial(), nil
	}
	var out Entitlements
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Entitlements{}, fmt.Errorf("decode entitlements: %w", err)
	}
	return out, nil
}

func (c *Client) freeTrial() Entitlements {
	products := make([]ProductEntitlement, 0, len(c.keys))
	for _, k := range c.keys {
		products = append(products, ProductEntitlement{Key: k, Plan: "free_trial", TrialStatus: "active", Credits: 20})
	}
	return Entitlements{Products: products}
}
