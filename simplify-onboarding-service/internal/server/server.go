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
	"github.com/simplify/onboarding/internal/userstore"
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

	// Shared platform directory — when wired, sessions become DocFlow-compatible
	// (internal user id + tenant + role) so products pick them up with no rewrite.
	var dir *userstore.Store
	if cfg.DatabaseURL != "" {
		d, derr := userstore.New(context.Background(), cfg.DatabaseURL, log)
		if derr != nil {
			log.Warn("shared directory unavailable — sessions won't carry DocFlow tenant/role", zap.Error(derr))
		} else {
			dir = d
			log.Info("shared platform directory connected — DocFlow-compatible sessions enabled")
		}
	}

	cat := catalog.New(cfg.ProductRegistryFile, log)
	keys := make([]string, 0, len(cat.List()))
	for _, p := range cat.List() {
		keys = append(keys, p.Key)
	}
	visitors := visitor.New(rdb, 30*24*time.Hour, cfg.CookieSecure, log)
	ent := entitlements.New(cfg.SimplifyCoreURL, cfg.SimplifyCoreToken, cfg.SimplifyCoreStub, keys, log)

	h := handler.New(handler.Deps{
		Cfg:          cfg,
		Log:          log,
		Users:        users,
		Dir:          dir,
		Sessions:     sessions,
		Catalog:      cat,
		Visitors:     visitors,
		Entitlements: ent,
	})

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(20 * time.Second))
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
		r.Get("/me", h.Me)
		r.Get("/logout", h.Logout)
		r.Get("/validate", h.Validate)
		r.Get("/clients", h.Clients)
		r.Get("/sso/{provider}", h.SSOStart)
		r.Post("/otp/email/start", h.ResendEmailCode) // resend the email code
		r.Post("/otp/email/verify", h.VerifyEmailCode)
		r.Post("/otp/mobile/start", h.StartMobileCode)
		r.Post("/otp/mobile/verify", h.VerifyMobileCode)
	})

	r.Route("/onb", func(r chi.Router) {
		r.Post("/session", h.OnbSession)
		r.Post("/event", h.OnbEvent)
		r.Get("/state", h.OnbState)
		r.Get("/products/{key}/motion", h.ProductMotion)
		r.Get("/entitlements", h.Entitlements)
	})

	return &Server{router: r}, nil
}
