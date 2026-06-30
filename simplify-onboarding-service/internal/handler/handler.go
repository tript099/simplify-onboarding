// Package handler wires the HTTP surface to the service's collaborators. The
// Handler struct owns every dependency; methods live in topic-specific files
// (auth.go, otp.go, onboarding.go, health.go).
package handler

import (
	"net/http"
	"time"

	"github.com/simplify/onboarding/internal/catalog"
	"github.com/simplify/onboarding/internal/config"
	"github.com/simplify/onboarding/internal/entitlements"
	"github.com/simplify/onboarding/internal/httpx"
	"github.com/redis/go-redis/v9"
	"github.com/simplify/onboarding/internal/identity"
	"github.com/simplify/onboarding/internal/notify"
	"github.com/simplify/onboarding/internal/scheduler"
	"github.com/simplify/onboarding/internal/session"
	"github.com/simplify/onboarding/internal/store"
	"github.com/simplify/onboarding/internal/visitor"
	"go.uber.org/zap"
)

// Handler holds the dependencies shared by all HTTP endpoints.
type Handler struct {
	cfg          *config.Config
	log          *zap.Logger
	rdb          *redis.Client
	users        identity.Provider
	db           *store.Store // onboarding's own DB (nil when not configured)
	sessions     *session.Service
	catalog      *catalog.Catalog
	visitors     *visitor.Service
	entitlements *entitlements.Client
	notify       *notify.Service   // demo/POC emails via MailForge (nil when unconfigured)
	scheduler    *scheduler.Client // meeting scheduler (nil/disabled when unconfigured)
	client       *http.Client      // outbound calls (Zitadel IDP intents for SSO)
}

// Deps groups everything Handler needs.
type Deps struct {
	Cfg          *config.Config
	Log          *zap.Logger
	RDB          *redis.Client
	Users        identity.Provider
	DB           *store.Store
	Sessions     *session.Service
	Catalog      *catalog.Catalog
	Visitors     *visitor.Service
	Entitlements *entitlements.Client
	Notify       *notify.Service
	Scheduler    *scheduler.Client
}

// New builds a Handler.
func New(d Deps) *Handler {
	return &Handler{
		cfg:          d.Cfg,
		log:          d.Log,
		rdb:          d.RDB,
		users:        d.Users,
		db:           d.DB,
		sessions:     d.Sessions,
		catalog:      d.Catalog,
		visitors:     d.Visitors,
		entitlements: d.Entitlements,
		notify:       d.Notify,
		scheduler:    d.Scheduler,
		client:       &http.Client{Timeout: 30 * time.Second},
	}
}

func bad(w http.ResponseWriter, msg string) {
	httpx.WriteError(w, http.StatusBadRequest, "bad_request", msg)
}
