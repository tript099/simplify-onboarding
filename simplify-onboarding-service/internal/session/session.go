// Package session manages HttpOnly browser sessions backed by Redis.
//
// The browser only ever holds an opaque session_id cookie; all identity and
// token material stays server-side. Sliding TTL with an absolute maximum, the
// same model docflow-auth uses, so behaviour is consistent across the platform.
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const cookieName = "session_id"

// Data is the value stored for every authenticated session. The JSON tags match
// docflow-auth's session schema so DocFlow (reading the shared Redis directly)
// picks up these sessions with no change. UserID is the platform-internal
// app_users.id; ZitadelSub keeps the external id for provider reconciliation.
type Data struct {
	UserID        string         `json:"user_id"`
	ZitadelSub    string         `json:"zitadel_sub,omitempty"`
	TenantID      string         `json:"tenant_id,omitempty"`
	Email         string         `json:"email"`
	FirstName     string         `json:"first_name,omitempty"`
	LastName      string         `json:"last_name,omitempty"`
	DisplayName   string         `json:"display_name,omitempty"`
	Phone         string         `json:"phone,omitempty"`
	Role          string         `json:"role,omitempty"`
	EmailVerified bool           `json:"email_verified"`
	PhoneVerified bool           `json:"phone_verified"`
	CreatedAt     time.Time      `json:"created_at"`
	Attrs         map[string]any `json:"attrs,omitempty"`
	// Demo marks the shared demo-account session. Products consume it normally (it's
	// a real SSO session), but the onboarding portal hides it — /me treats a demo
	// session as signed-out so the portal never shows a "logged in" demo user.
	Demo bool `json:"demo,omitempty"`
}

// Service stores and retrieves sessions in Redis.
type Service struct {
	rdb        *redis.Client
	slidingTTL time.Duration
	absMax     time.Duration
	persist    bool // when true: session never expires on its own — only logout removes it
	secure     bool
	domain     string
	log        *zap.Logger
}

// persistCookieMaxAge is the cookie lifetime in persist mode (long; the server
// session is authoritative and only logout clears it).
const persistCookieMaxAge = 365 * 24 * 60 * 60 // 1 year, in seconds

// NewService builds a session service.
func NewService(rdb *redis.Client, sliding, absMax time.Duration, persist, secure bool, domain string, log *zap.Logger) *Service {
	return &Service{rdb: rdb, slidingTTL: sliding, absMax: absMax, persist: persist, secure: secure, domain: domain, log: log}
}

// Create stores a new session and sets the HttpOnly cookie.
func (s *Service) Create(ctx context.Context, w http.ResponseWriter, data Data) (string, error) {
	data.CreatedAt = time.Now().UTC()
	id := uuid.NewString()

	raw, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal session: %w", err)
	}
	// persist → no Redis expiry (only logout removes it); otherwise sliding TTL.
	storeTTL := s.slidingTTL
	cookieMaxAge := int(s.absMax.Seconds())
	if s.persist {
		storeTTL = 0 // 0 = no expiry
		cookieMaxAge = persistCookieMaxAge
	}
	if err := s.rdb.Set(ctx, key(id), raw, storeTTL).Err(); err != nil {
		return "", fmt.Errorf("store session: %w", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    id,
		Path:     "/",
		Domain:   s.domain,
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
	s.log.Info("session created", zap.String("user_id", data.UserID), zap.Bool("persist", s.persist))
	return id, nil
}

// Get fetches a session and slides its TTL, enforcing the absolute maximum age.
func (s *Service) Get(ctx context.Context, id string) (*Data, error) {
	raw, err := s.rdb.Get(ctx, key(id)).Bytes()
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	var data Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	// In persist mode the session never expires on its own — skip the absolute-max
	// check. Crucially, HEAL the key back to no-expiry: this is a SHARED session and
	// another service on the same Redis (DocFlow's BFF) re-arms a sliding TTL on every
	// read. Without this, that TTL would eventually expire the session and sign the
	// user out of the portal too. PERSIST strips any TTL so it lives until logout.
	if !s.persist {
		if time.Since(data.CreatedAt) > s.absMax {
			_ = s.rdb.Del(ctx, key(id)).Err()
			return nil, fmt.Errorf("session expired (absolute max)")
		}
		_ = s.rdb.Expire(ctx, key(id), s.slidingTTL).Err()
	} else {
		_ = s.rdb.Persist(ctx, key(id)).Err()
	}
	return &data, nil
}

// FromRequest resolves the session referenced by the request cookie.
func (s *Service) FromRequest(r *http.Request) (*Data, string, error) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil, "", err
	}
	data, err := s.Get(r.Context(), c.Value)
	if err != nil {
		return nil, "", err
	}
	return data, c.Value, nil
}

// Update applies mutate to a stored session, preserving its remaining TTL.
func (s *Service) Update(ctx context.Context, id string, mutate func(*Data)) error {
	raw, err := s.rdb.Get(ctx, key(id)).Bytes()
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}
	var data Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("unmarshal session: %w", err)
	}
	mutate(&data)
	updated, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	if s.persist {
		return s.rdb.Set(ctx, key(id), updated, 0).Err() // keep no-expiry
	}
	ttl, err := s.rdb.TTL(ctx, key(id)).Result()
	if err != nil || ttl <= 0 {
		ttl = s.slidingTTL
	}
	return s.rdb.SetEx(ctx, key(id), updated, ttl).Err()
}

// Delete removes a session and clears the cookie.
func (s *Service) Delete(ctx context.Context, w http.ResponseWriter, id string) {
	_ = s.rdb.Del(ctx, key(id)).Err()
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		Domain:   s.domain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func key(id string) string { return "session:" + id }
