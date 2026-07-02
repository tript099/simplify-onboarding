// Package server wires the dependency graph and builds the chi router.
package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/simplify/onboarding/internal/catalog"
	"github.com/simplify/onboarding/internal/config"
	"github.com/simplify/onboarding/internal/entitlements"
	"github.com/simplify/onboarding/internal/handler"
	"github.com/simplify/onboarding/internal/identity"
	"github.com/simplify/onboarding/internal/session"
	"github.com/simplify/onboarding/internal/mailforge"
	"github.com/simplify/onboarding/internal/notify"
	"github.com/simplify/onboarding/internal/scheduler"
	"github.com/simplify/onboarding/internal/store"
	"github.com/simplify/onboarding/internal/visitor"
)

// Server exposes the built router.
type Server struct {
	router http.Handler
}

// Router returns the HTTP handler.
func (s *Server) Router() http.Handler { return s.router }

// New assembles every collaborator and returns a ready Server.
func New(cfg *config.Config, log *zap.Logger) (*Server, error) {
	opt, err := redis.ParseURL(fmt.Sprintf("%s/%d", strings.TrimRight(cfg.RedisURL, "/"), cfg.RedisDB))
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	rdb := redis.NewClient(opt)

	// ── Identity provider ─────────────────────────────────────────
	var users identity.Provider
	switch cfg.IdentityProvider {
	case "dev":
		log.Warn("identity provider: dev (redis + bcrypt) — OFFLINE DEV ONLY, not for production")
		users = identity.NewDevProvider(rdb, cfg.CodeTTL, cfg.DebugReturnCode, log)
	default:
		log.Info("identity provider: zitadel", zap.String("issuer", cfg.ZitadelInternalURL))
		if cfg.ZitadelServicePAT == "" {
			log.Warn("ZITADEL_SERVICE_PAT not set — registration/login will be unavailable")
		}
		users = identity.NewZitadelProvider(identity.ZitadelConfig{
			Issuer:     cfg.ZitadelInternalURL,
			ServicePAT: cfg.ZitadelServicePAT,
			OrgID:      cfg.ZitadelOrgID,
			ReturnCode: cfg.DebugReturnCode,
		}, log)
	}

	sessions := session.NewService(rdb, cfg.SessionTTL, cfg.SessionAbsMax, cfg.SessionPersist, cfg.CookieSecure, cfg.CookieDomain, log)
	if cfg.SessionPersist {
		log.Info("session mode: persist-until-logout (central SSO session)")
	}

	// Onboarding's OWN database (users, demo requests, events, audit). Migrations run
	// at startup. Separate from the shared platform DB above.
	var db *store.Store
	if cfg.OnboardingDatabaseURL != "" {
		s, derr := store.New(context.Background(), cfg.OnboardingDatabaseURL, log)
		if derr != nil {
			log.Error("onboarding database unavailable — persistence disabled", zap.Error(derr))
		} else {
			db = s
		}
	} else {
		log.Warn("ONBOARDING_DATABASE_URL not set — user/demo/event persistence disabled")
	}

	cat := catalog.New(cfg.ProductRegistryFile, log)
	keys := make([]string, 0, len(cat.List()))
	for _, p := range cat.List() {
		keys = append(keys, p.Key)
	}
	visitors := visitor.New(rdb, 30*24*time.Hour, cfg.CookieSecure, log)
	ent := entitlements.New(cfg.SimplifyCoreURL, cfg.SimplifyCoreToken, cfg.SimplifyCoreStub, keys, log)

	// Email (MailForge) — demo/POC confirmation + team notification. Email HTML lives
	// in MailForge as published templates; we send by template_id + variables.
	mf := mailforge.New(cfg.MailforgeURL, cfg.MailforgeAPIKey)
	notifier := notify.New(mf, cfg.DemoRecipientsFile, notify.Config{
		FromName:               cfg.MailforgeFromName,
		FromEmail:              cfg.MailforgeFromEmail,
		ReplyTo:                cfg.MailforgeReplyTo,
		PortalURL:              cfg.FrontendURL,
		ConfirmationTemplateID: cfg.MailforgeTplConfirmation,
		TeamNotifyTemplateID:   cfg.MailforgeTplTeamNotify,
		InviteTemplateID:       cfg.MailforgeTplInvite,
	}, log)
	if mf.Enabled() {
		log.Info("notify: MailForge email enabled", zap.String("from", cfg.MailforgeFromEmail))
	} else {
		log.Warn("notify: MAILFORGE_API_KEY not set — demo/POC emails will be skipped")
	}

	// External meeting scheduler (Google Meet / Teams).
	sched := scheduler.New(cfg.SchedulerURL, cfg.SchedulerPath, cfg.SchedulerPasscode, cfg.SchedulerDuration)
	if sched.Enabled() {
		log.Info("scheduler: meeting scheduling enabled", zap.String("path", cfg.SchedulerPath))
	} else {
		log.Warn("scheduler: SCHEDULER_URL/PASSCODE not set — meeting scheduling disabled")
	}

	h := handler.New(handler.Deps{
		Cfg:          cfg,
		Log:          log,
		RDB:          rdb,
		Users:        users,
		DB:           db,
		Sessions:     sessions,
		Catalog:      cat,
		Visitors:     visitors,
		Entitlements: ent,
		Notify:       notifier,
		Scheduler:    sched,
	})

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	// Request budget for the whole flow (kept below the server WriteTimeout so the
	// app returns a clean 504 rather than the connection being hard-closed). Sign-in
	// fans out to Zitadel + the shared DB, which is slow over the VPN.
	r.Use(chimw.Timeout(45 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-ID", "X-Internal-Secret"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", h.Health)
	r.Get("/ready", h.Ready)

	// Session validation for products / Kong (docflow-auth `/validate` contract).
	r.Get("/validate", h.Validate)

	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", h.Register)
		r.Post("/login", h.SignIn)
		r.Post("/demo", h.DemoLogin) // "Try it now" — shared demo account, no signup
		r.Get("/me", h.Me)
		r.Get("/logout", h.Logout)
		r.Post("/password/forgot", h.ForgotPassword) // email a reset link
		r.Post("/password/reset", h.ResetPassword)   // complete reset via emailed code
		r.Get("/validate", h.Validate)
		r.Get("/clients", h.Clients)
		r.Get("/sso/{provider}", h.SSOStart)
		r.Get("/sso/callback/{provider}", h.SSOCallback) // Zitadel IDP-intent return
		r.Post("/otp/email/start", h.ResendEmailCode) // resend the email code
		r.Post("/otp/email/verify", h.VerifyEmailCode)
		r.Post("/otp/mobile/start", h.StartMobileCode)
		r.Post("/otp/mobile/verify", h.VerifyMobileCode)
		// Passwordless sign-in (code to email/mobile).
		r.Post("/login/otp/start", h.StartLoginOTP)
		r.Post("/login/otp/verify", h.VerifyLoginOTP)
	})

	r.Route("/onb", func(r chi.Router) {
		r.Post("/session", h.OnbSession)
		r.Post("/event", h.OnbEvent)
		r.Get("/state", h.OnbState)
		r.Get("/products/{key}/motion", h.ProductMotion)
		r.Get("/entitlements", h.Entitlements)
		r.Post("/demo", h.SubmitDemo)
		r.Post("/demo/{id}/schedule", h.ScheduleDemo) // internal: book a meeting for a request
	})

	return &Server{router: r}, nil
}
