// Package middleware holds cross-cutting HTTP middleware specific to this service.
package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/simplify/onboarding/internal/httpx"
)

// InternalSecret guards service-to-service routes with a shared secret header.
// If no secret is configured the guard denies all traffic (fail closed).
func InternalSecret(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := r.Header.Get("X-Internal-Secret")
			if secret == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "internal auth required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
