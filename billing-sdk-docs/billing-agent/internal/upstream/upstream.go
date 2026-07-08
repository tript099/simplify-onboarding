// Package upstream is the typed client for SimplifyBilling core's /api/v1/sdk/*
// surface. It injects the Bearer credential from the configured Authenticator.
package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/simplify/billing-agent/internal/auth"
)

type Client struct {
	baseURL string
	hc      *http.Client
	auth    auth.Authenticator
}

func New(baseURL string, a auth.Authenticator) *Client {
	return &Client{baseURL: baseURL, hc: &http.Client{Timeout: 5 * time.Second}, auth: a}
}

func (c *Client) AuthKind() string { return c.auth.Kind() }

// ── Types (subset of the entitlement snapshot + consume result) ──────────────

type Entitlement struct {
	OrgID         string `json:"org_id"`
	AppID         string `json:"app_id"`
	PlanCode      string `json:"plan_code"`
	PlanName      string `json:"plan_name"`
	SubState      string `json:"subscription_state"`
	CurrentPeriod *struct {
		Start time.Time `json:"start"`
		End   time.Time `json:"end"`
	} `json:"current_period"`
	WalletBalance int64     `json:"wallet_balance_cents"`
	Currency      string    `json:"currency"`
	Features      []Feature `json:"features"`
}

type Feature struct {
	Code          string `json:"code"`
	Enabled       bool   `json:"enabled"`
	IncludedQuota *int64 `json:"included_quota"`
	PeriodQuota   *int64 `json:"period_quota"`
	Remaining     *int64 `json:"remaining"`
	TotalQuota    *int64   `json:"total_effective_quota"`
	CanConsume    *bool    `json:"can_consume"`
	FundingChain  []string `json:"funding_chain"` // configured order: SUBSCRIPTION_QUOTA→ADDON_GRANTS→TOKEN_POOL→WALLET→…
	ResetAt       string   `json:"reset_at"`
}

func (e Entitlement) Find(code string) (Feature, bool) {
	for _, f := range e.Features {
		if equalFold(f.Code, code) {
			return f, true
		}
	}
	return Feature{}, false
}

type ConsumeResult struct {
	Allowed        bool             `json:"allowed"`
	Reason         string           `json:"reason"`
	UnitsConsumed  int64            `json:"units_consumed"`
	FundingSource  string           `json:"funding_source"`
	PaymentOptions []map[string]any `json:"payment_options"`
}

// ── Calls ────────────────────────────────────────────────────────────────────

func (c *Client) GetEntitlement(ctx context.Context, org string) (Entitlement, int, error) {
	var e Entitlement
	status, err := c.do(ctx, http.MethodGet, "/api/v1/sdk/entitlements/"+org, nil, &e)
	return e, status, err
}

func (c *Client) Consume(ctx context.Context, org, feature string, units int64, idem string) (ConsumeResult, int, error) {
	var res ConsumeResult
	status, err := c.do(ctx, http.MethodPost, "/api/v1/sdk/metering/consume", map[string]any{
		"org_id": org, "feature_code": feature, "units": units, "idempotency_key": idem,
	}, &res)
	return res, status, err
}

// BatchEvent is one buffered usage event flushed to the batch endpoint.
type BatchEvent struct {
	OrgID          string    `json:"org_id"`
	FeatureCode    string    `json:"feature_code"`
	Units          int64     `json:"units"`
	IdempotencyKey string    `json:"idempotency_key"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// ConsumeBatch flushes buffered usage events to core's idempotent batch endpoint.
func (c *Client) ConsumeBatch(ctx context.Context, events []BatchEvent) (int, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/sdk/metering/consume/batch", map[string]any{"events": events}, nil)
}

// Post proxies an arbitrary /sdk POST (provisioning) and returns the raw body.
func (c *Client) Post(ctx context.Context, path string, body, out any) (int, error) {
	return c.do(ctx, http.MethodPost, path, body, out)
}

// Ping is a cheap reachability probe for readiness.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.do(ctx, http.MethodGet, "/api/v1/sdk/entitlements/00000000-0000-0000-0000-000000000000", nil, nil)
	return err
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) (int, error) {
	tok, err := c.auth.Token(ctx)
	if err != nil {
		return 0, err
	}
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil && len(raw) > 0 {
		if uerr := json.Unmarshal(raw, out); uerr != nil {
			return resp.StatusCode, errors.New("decode: " + uerr.Error())
		}
	}
	return resp.StatusCode, nil
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
