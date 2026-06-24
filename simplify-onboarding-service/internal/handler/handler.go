// Package handler wires the HTTP surface to the service's collaborators. The
// Handler struct owns every dependency; methods live in topic-specific files
// (auth.go, otp.go, onboarding.go, health.go).
package handler

import (
	"net/http"

	"github.com/simplify/onboarding/internal/catalog"
	"github.com/simplify/onboarding/internal/config"
	"github.com/simplify/onboarding/internal/entitlements"
	"github.com/simplify/onboarding/internal/httpx"
	"github.com/simplify/onboarding/internal/identity"
	"github.com/simplify/onboarding/internal/session"
	"github.com/simplify/onboarding/internal/userstore"
	"github.com/simplify/onboarding/internal/visitor"
	"go.uber.org/zap"
)

// Handler holds the dependencies shared by all HTTP endpoints.
type Handler struct {
	cfg          *config.Config
	log          *zap.Logger
	users        identity.Provider
	dir          *userstore.Store // shared platform directory (nil when no DB)
	sessions     *session.Service
	catalog      *catalog.Catalog
	visitors     *visitor.Service
	entitlements *entitlements.Client
}

// Deps groups everything Handler needs.
type Deps struct {
	Cfg          *config.Config
	Log          *zap.Logger
	Users        identity.Provider
	Dir          *userstore.Store
	Sessions     *session.Service
	Catalog      *catalog.Catalog
	Visitors     *visitor.Service
	Entitlements *entitlements.Client
}

// New builds a Handler.
func New(d Deps) *Handler {
	return &Handler{
		cfg:          d.Cfg,
		log:          d.Log,
		users:        d.Users,
		dir:          d.Dir,
		sessions:     d.Sessions,
		catalog:      d.Catalog,
		visitors:     d.Visitors,
		entitlements: d.Entitlements,
	}
}

func bad(w http.ResponseWriter, msg string) {
	httpx.WriteError(w, http.StatusBadRequest, "bad_request", msg)
}
