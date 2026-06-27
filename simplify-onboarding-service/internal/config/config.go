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

	// OnboardingDatabaseURL — this service's OWN database (users, demo requests,
	// onboarding events, login audit). Separate from the shared platform DB: the
	// onboarding service owns this data. When set, the store + migrations come up.
	OnboardingDatabaseURL string

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

	// IDPs maps provider name → Zitadel IDP ID for federated SSO (Google/Microsoft).
	// Auto-discovered from IDP_<NAME>_ID env vars (IDP_GOOGLE_ID → IDPs["google"]).
	IDPs map[string]string

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

	// MailForge — the shared email service (same one DocFlow uses). Accessed over
	// HTTP with a project-scoped API key; sends demo/POC confirmation + team notify.
	MailforgeURL       string
	MailforgeAPIKey    string
	MailforgeFromName  string
	MailforgeFromEmail string
	MailforgeReplyTo   string
	// MailForge template ids (published collections under the onboarding project).
	MailforgeTplConfirmation string
	MailforgeTplTeamNotify   string
	MailforgeTplInvite       string
	// DemoRecipientsFile — JSON list of Simplify-team addresses to notify, routable by
	// request type / product (UI-configurable later).
	DemoRecipientsFile string

	// Scheduler — the external Google Meet / Teams scheduling API. POST {SchedulerPath}
	// with {attendees, start_time, duration, passcode} → { meeting_url }.
	SchedulerURL      string
	SchedulerPath     string // /schedule_google (default) | /schedule_teams
	SchedulerPasscode string
	SchedulerDuration int // default meeting length (minutes)

	// Demo account — "Try it now" signs in as this shared user (no signup). Because
	// it's a real user on the shared SSO session, the demo works across all products.
	DemoEnabled   bool
	DemoEmail     string
	DemoPassword  string
	DemoFirstName string
	DemoLastName  string

	ProductRegistryFile string

	CORSAllowedOrigins []string
	InternalSecret     string
}

// Load reads configuration from the environment.
func Load() *Config {
	appEnv := envOr("APP_ENV", "development")
	return &Config{
		IDPs: discoverIDPs(),
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

		OnboardingDatabaseURL: os.Getenv("ONBOARDING_DATABASE_URL"),

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

		MailforgeURL:       os.Getenv("MAILFORGE_URL"),
		MailforgeAPIKey:    os.Getenv("MAILFORGE_API_KEY"),
		MailforgeFromName:  envOr("MAILFORGE_FROM_NAME", "Simplify"),
		MailforgeFromEmail: envOr("MAILFORGE_FROM_EMAIL", "no-reply@simplifyai.id"),
		MailforgeReplyTo:   envOr("MAILFORGE_REPLY_TO", "sales@simplifyaipro.com"),

		MailforgeTplConfirmation: os.Getenv("MAILFORGE_TEMPLATE_CONFIRMATION"),
		MailforgeTplTeamNotify:   os.Getenv("MAILFORGE_TEMPLATE_TEAM_NOTIFY"),
		MailforgeTplInvite:       os.Getenv("MAILFORGE_TEMPLATE_INVITE"),

		DemoRecipientsFile: envOr("DEMO_RECIPIENTS_FILE", "config/demo-recipients.json"),

		SchedulerURL:      os.Getenv("SCHEDULER_URL"),
		SchedulerPath:     envOr("SCHEDULER_PATH", "/schedule_google"),
		SchedulerPasscode: os.Getenv("SCHEDULER_PASSCODE"),
		SchedulerDuration: envInt("SCHEDULER_DURATION_MIN", 30),

		DemoEnabled:   envBool("DEMO_ENABLED", false),
		DemoEmail:     envOr("DEMO_EMAIL", "demo@simplifyai.id"),
		DemoPassword:  os.Getenv("DEMO_PASSWORD"),
		DemoFirstName: envOr("DEMO_FIRST_NAME", "Simplify"),
		DemoLastName:  envOr("DEMO_LAST_NAME", "Demo"),

		ProductRegistryFile: os.Getenv("PRODUCT_REGISTRY_FILE"),

		CORSAllowedOrigins: splitTrim(envOr("CORS_ALLOWED_ORIGINS", "http://localhost:3100"), ","),
		InternalSecret:     os.Getenv("INTERNAL_JWT_SECRET"),
	}
}

// discoverIDPs builds the provider→Zitadel-IDP-ID map from IDP_<NAME>_ID env vars.
func discoverIDPs() map[string]string {
	idps := map[string]string{}
	for _, e := range os.Environ() {
		kv := strings.SplitN(e, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := kv[0], kv[1]
		if strings.HasPrefix(key, "IDP_") && strings.HasSuffix(key, "_ID") && val != "" {
			name := strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(key, "IDP_"), "_ID"))
			if name != "" {
				idps[name] = val
			}
		}
	}
	return idps
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
