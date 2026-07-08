// Package account calls Core's HMAC-signed Account API to get a user's per-product
// subscriptions (keyed by email). See ACCOUNT_API.md.
package account

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Subscription is one product subscription on an org (fields per ACCOUNT_API.md).
type Subscription struct {
	ProductSlug        string `json:"product_slug"`
	ProductName        string `json:"product_name"`
	PlanCode           string `json:"plan_code"`
	PlanName           string `json:"plan_name"`
	BillingPeriod      string `json:"billing_period"`
	State              string `json:"state"`
	PriceCents         int64  `json:"price_cents"`
	Currency           string `json:"currency"`
	AutoRenew          bool   `json:"auto_renew"`
	CancelAtPeriodEnd  bool   `json:"cancel_at_period_end"`
	CurrentPeriodStart string `json:"current_period_start"`
	CurrentPeriodEnd   string `json:"current_period_end"`
}

// Org is one organisation the user belongs to.
type Org struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Slug          string         `json:"slug"`
	Type          string         `json:"type"`
	Subscriptions []Subscription `json:"subscriptions"`
}

type orgsResponse struct {
	Email string `json:"email"`
	Orgs  []Org  `json:"orgs"`
}

// Client is the HMAC-signed Account API client.
type Client struct {
	base   string
	keyID  string
	secret string
	http   *http.Client
	log    *zap.Logger
}

// New builds the client. base like https://api-dev.simplifyaipro.com. Disabled (Enabled()
// false) if key/secret/base are missing.
func New(base, keyID, secret string, log *zap.Logger) *Client {
	return &Client{
		base:   strings.TrimRight(base, "/"),
		keyID:  keyID,
		secret: secret,
		http:   &http.Client{Timeout: 8 * time.Second},
		log:    log,
	}
}

// Enabled reports whether the client is configured.
func (c *Client) Enabled() bool { return c.base != "" && c.keyID != "" && c.secret != "" }

// OrgsByEmail returns the orgs (with subscriptions) for a user email.
func (c *Client) OrgsByEmail(ctx context.Context, email string) ([]Org, error) {
	body, _ := json.Marshal(map[string]string{"email": email})
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	mac := hmac.New(sha256.New, []byte(c.secret))
	mac.Write([]byte(ts + "." + string(body)))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/api/v1/public/account/orgs", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Key", c.keyID)
	req.Header.Set("X-Account-Timestamp", ts)
	req.Header.Set("X-Account-Signature", sig)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("account api request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("account api status %d", resp.StatusCode)
	}
	var out orgsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("account api decode: %w", err)
	}
	return out.Orgs, nil
}
