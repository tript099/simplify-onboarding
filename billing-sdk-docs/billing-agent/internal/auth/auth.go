// Package auth provides the pluggable upstream authenticator: a static product
// API key, or Zitadel machine-to-machine (client-credentials) tokens. The agent
// only ever asks for "the Bearer value to send" — swapping auth is config-only.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/simplify/billing-agent/internal/config"
)

// Authenticator returns the Bearer credential to present to core /sdk.
type Authenticator interface {
	Token(ctx context.Context) (string, error)
	Kind() string
}

// New picks the authenticator from config: Zitadel if configured, else the static key.
func New(cfg config.Config) (Authenticator, error) {
	if cfg.UseZitadel() {
		return &zitadel{cfg: cfg.Zitadel, hc: &http.Client{Timeout: 5 * time.Second}}, nil
	}
	if cfg.APIKey == "" {
		return nil, errors.New("BILLING_API_KEY or ZITADEL_* must be configured")
	}
	return staticKey{key: cfg.APIKey}, nil
}

// staticKey returns the same product API key every time.
type staticKey struct{ key string }

func (s staticKey) Token(context.Context) (string, error) { return s.key, nil }
func (staticKey) Kind() string                            { return "api_key" }

// zitadel fetches short-lived access tokens via OAuth2 client-credentials and
// caches them until ~60s before expiry. This gives products central rotation /
// revocation and avoids long-lived secrets on the wire.
type zitadel struct {
	cfg config.Zitadel
	hc  *http.Client

	mu    sync.Mutex
	token string
	exp   time.Time
}

func (z *zitadel) Kind() string { return "zitadel_m2m" }

func (z *zitadel) Token(ctx context.Context) (string, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	if z.token != "" && time.Now().Before(z.exp) {
		return z.token, nil
	}
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {z.cfg.ClientID},
		"client_secret": {z.cfg.ClientSecret},
		"scope":         {z.cfg.Scope},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, z.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := z.hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", errors.New("zitadel token endpoint: " + resp.Status)
	}
	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.AccessToken == "" {
		return "", errors.New("zitadel token endpoint returned no access_token")
	}
	ttl := time.Duration(body.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 55 * time.Minute
	}
	z.token = body.AccessToken
	z.exp = time.Now().Add(ttl - 60*time.Second)
	return z.token, nil
}
