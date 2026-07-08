// Package config loads the billing-agent configuration from the environment.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Zitadel struct {
	TokenURL     string // OAuth2 token endpoint (Zitadel) — set to enable JWT/M2M auth
	ClientID     string
	ClientSecret string
	Scope        string // e.g. "openid urn:zitadel:iam:org:project:id:<projID>:aud"
}

type Config struct {
	BaseURL  string        // SimplifyBilling core
	Listen   string        // local API listen addr
	CacheTTL time.Duration // entitlement cache freshness
	FailOpen bool          // when core unreachable: allow (true) or deny (false)

	APIKey  string  // static product API key (bk_…) — used when Zitadel is not configured
	Zitadel Zitadel // when set, the agent fetches short-lived M2M tokens instead of a static key

	// Consumption write-back (report path): free-quota usage is buffered and
	// flushed to core's batch endpoint every FlushInterval or FlushSize events.
	// authorize stays synchronous (binding gate). 0 size/interval disables batching.
	FlushSize     int
	FlushInterval time.Duration
}

// UseZitadel reports whether Zitadel client-credentials auth is configured.
func (c Config) UseZitadel() bool {
	return c.Zitadel.TokenURL != "" && c.Zitadel.ClientID != ""
}

func Load() Config {
	ttl, _ := time.ParseDuration(env("AGENT_CACHE_TTL", "45s"))
	flush, _ := time.ParseDuration(env("AGENT_FLUSH_INTERVAL", "2s"))
	return Config{
		FlushSize:     atoi(env("AGENT_FLUSH_SIZE", "50")),
		FlushInterval: flush,
		BaseURL:  strings.TrimRight(env("BILLING_BASE_URL", "http://core:8081"), "/"),
		Listen:   env("AGENT_LISTEN", ":8099"),
		CacheTTL: ttl,
		FailOpen: strings.EqualFold(env("AGENT_FAIL_MODE", "closed"), "open"),
		APIKey:   env("BILLING_API_KEY", ""),
		Zitadel: Zitadel{
			TokenURL:     env("ZITADEL_TOKEN_URL", ""),
			ClientID:     env("ZITADEL_CLIENT_ID", ""),
			ClientSecret: env("ZITADEL_CLIENT_SECRET", ""),
			Scope:        env("ZITADEL_SCOPE", "openid"),
		},
	}
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
