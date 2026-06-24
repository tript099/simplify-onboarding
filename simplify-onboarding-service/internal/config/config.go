// Package config loads all runtime configuration from environment variables.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds every runtime knob for the onboarding service.
type Config struct {
	Port   string
	AppEnv string // development | staging | production

	FrontendURL    string
	GatewayBaseURL string

	// Redis — sessions, codes (dev provider) and visitor state.
	RedisURL      string
	RedisDB       int
	SessionTTL    time.Duration
	SessionAbsMax time.Duration
	// SessionPersist: the central SSO session never expires on its own — only
	// logout removes it (true silent SSO across products).
	SessionPersist bool
	CookieSecure   bool
	CookieDomain   string

	// DatabaseURL — the SHARED platform DB (app_users / tenants). When set, the
	// session is written with the DocFlow schema (internal user id + tenant + role)
	// so products reading the shared Redis pick it up directly.
	DatabaseURL string

	// Identity backend: "zitadel" (production — owns passwords + sends emails) or
	// "dev" (self-contained Redis+bcrypt, offline only).
	IdentityProvider string

	// Zitadel.
	ZitadelIssuer      string
	ZitadelExternalURL string
	ZitadelInternalURL string
	ZitadelClientID    string
	ZitadelServicePAT  string
	ZitadelOrgID       string

	// Verification.
	CodeTTL               time.Duration // dev provider code lifetime
	ResendCooldownSeconds int           // advisory cooldown surfaced to the UI
	// DebugReturnCode (DEBUG_RETURN_CODE) makes the provider RETURN the verification
	// code via the API instead of only emailing it, and exposes it in responses.
	// Local testing only — must be false in production.
	DebugReturnCode bool

	// SimplifyCore (entitlements). Stub returns a free trial until Core ships.
	SimplifyCoreURL   string
	SimplifyCoreToken string
	SimplifyCoreStub  bool

	ProductRegistryFile string

	CORSAllowedOrigins []string
	InternalSecret     string
}

// Load reads configuration from the environment.
func Load() *Config {
	appEnv := envOr("APP_ENV", "development")
	return &Config{
		Port:   envOr("PORT", "8090"),
		AppEnv: appEnv,

		FrontendURL:    envOr("FRONTEND_URL", "http://localhost:3100"),
		GatewayBaseURL: envOr("GATEWAY_BASE_URL", "http://localhost:8000"),

		RedisURL:      envOr("REDIS_URL", "redis://localhost:6379"),
		RedisDB:       envInt("REDIS_DB", 1),
		SessionTTL:     envDuration("SESSION_TTL", 30*time.Minute),
		SessionAbsMax:  envDuration("SESSION_ABS_MAX", 8*time.Hour),
		SessionPersist: envBool("SESSION_PERSIST", true),
		CookieSecure:   envBool("COOKIE_SECURE", appEnv == "production"),
		CookieDomain:   os.Getenv("COOKIE_DOMAIN"),

		DatabaseURL: os.Getenv("DATABASE_URL"),

		IdentityProvider: envOr("IDENTITY_PROVIDER", "zitadel"),

		ZitadelIssuer:      os.Getenv("ZITADEL_ISSUER"),
		ZitadelExternalURL: envOr("ZITADEL_EXTERNAL_URL", os.Getenv("ZITADEL_ISSUER")),
		ZitadelInternalURL: envOr("ZITADEL_INTERNAL_URL", os.Getenv("ZITADEL_ISSUER")),
		ZitadelClientID:    os.Getenv("ZITADEL_CLIENT_ID"),
		ZitadelServicePAT:  os.Getenv("ZITADEL_SERVICE_PAT"),
		ZitadelOrgID:       os.Getenv("ZITADEL_ORG_ID"),

		CodeTTL:               envDuration("CODE_TTL", 10*time.Minute),
		ResendCooldownSeconds: envInt("RESEND_COOLDOWN_SECONDS", 30),
		DebugReturnCode:       envBool("DEBUG_RETURN_CODE", false),

		SimplifyCoreURL:   os.Getenv("SIMPLIFYCORE_BASE_URL"),
		SimplifyCoreToken: os.Getenv("SIMPLIFYCORE_TOKEN"),
		SimplifyCoreStub:  envBool("SIMPLIFYCORE_STUB", true),

		ProductRegistryFile: os.Getenv("PRODUCT_REGISTRY_FILE"),

		CORSAllowedOrigins: splitTrim(envOr("CORS_ALLOWED_ORIGINS", "http://localhost:3100"), ","),
		InternalSecret:     os.Getenv("INTERNAL_JWT_SECRET"),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	switch strings.ToLower(os.Getenv(key)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func splitTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
