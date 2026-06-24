package handler

import (
	"net/http"

	"github.com/simplify/onboarding/internal/httpx"
)

// Health is a liveness probe.
func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "simplify-onboarding-service"})
}

// Ready is a readiness probe (extend with Redis ping if desired).
func (h *Handler) Ready(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ready": true})
}
