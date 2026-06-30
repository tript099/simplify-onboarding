package handler

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/simplify/onboarding/internal/httpx"
	"github.com/simplify/onboarding/internal/identity"
	"github.com/simplify/onboarding/internal/session"
	"github.com/simplify/onboarding/internal/store"
	"go.uber.org/zap"
)

type registerRequest struct {
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Company     string `json:"company"`
	JobTitle    string `json:"jobTitle"`
	Password    string `json:"password"`
	Consent     bool   `json:"consent"`
}

type registerResponse struct {
	VerificationID string `json:"verificationId"`
	Email          string `json:"email"`
	// DebugCode is set ONLY in dev/testing (DEBUG_RETURN_CODE) — never in production.
	DebugCode string `json:"debugCode,omitempty"`
}

// Register creates the account in the identity provider (which emails the
// verification code) and signs the user in immediately. Email stays unverified
// until the code is confirmed — sensitive actions gate on it (verify-late).
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		bad(w, "invalid request body")
		return
	}
	req.Email = identity.NormalizeEmail(req.Email)

	switch {
	case req.FirstName == "" || req.LastName == "":
		bad(w, "first and last name are required")
		return
	case !strings.Contains(req.Email, "@"):
		bad(w, "a valid work email is required")
		return
	case len(req.Password) < 8:
		bad(w, "password must be at least 8 characters")
		return
	case !req.Consent:
		httpx.WriteError(w, http.StatusBadRequest, "consent_required", "please accept the privacy policy to continue")
		return
	}

	user, debugCode, err := h.users.Register(r.Context(), identity.RegisterInput{
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		DisplayName: req.DisplayName,
		Email:       req.Email,
		Phone:       req.Phone,
		Company:     req.Company,
		JobTitle:    req.JobTitle,
		Password:    req.Password,
	})
	if errors.Is(err, identity.ErrEmailTaken) {
		httpx.WriteError(w, http.StatusConflict, "email_taken", "An account with this email already exists.")
		return
	}
	if errors.Is(err, identity.ErrNotConfigured) {
		httpx.WriteError(w, http.StatusServiceUnavailable, "provider_unavailable", "Registration is not configured. Please contact your administrator.")
		return
	}
	if err != nil {
		h.log.Error("register failed", zap.Error(err))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not create your account")
		return
	}

	// Carry the registration profile (Zitadel doesn't return company/title) so the
	// onboarding DB captures the full sign-up, not just the identity.
	user.Company = req.Company
	user.JobTitle = req.JobTitle
	if user.Phone == "" {
		user.Phone = req.Phone
	}
	if err := h.establishSession(w, r, user, false, "password"); err != nil {
		h.log.Error("register: create session", zap.Error(err))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not start your session")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, registerResponse{
		VerificationID: user.ID,
		Email:          user.Email,
		DebugCode:      h.debug(debugCode),
	})
}

type signInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type nextResponse struct {
	Next string `json:"next"`
}

// SignIn verifies credentials with the identity provider and starts the session.
func (h *Handler) SignIn(w http.ResponseWriter, r *http.Request) {
	var req signInRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		bad(w, "invalid request body")
		return
	}
	user, err := h.users.Authenticate(r.Context(), req.Email, req.Password)
	if errors.Is(err, identity.ErrInvalidCredentials) {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "Incorrect email or password.")
		return
	}
	if errors.Is(err, identity.ErrNotConfigured) {
		httpx.WriteError(w, http.StatusServiceUnavailable, "provider_unavailable", "auth service not configured")
		return
	}
	if err != nil {
		h.log.Error("authenticate failed", zap.Error(err))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not sign you in")
		return
	}
	if err := h.establishSession(w, r, user, false, "password"); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not start your session")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, nextResponse{Next: "/"})
}

// Me returns the current user from the session cookie. Verification flags are a
// snapshot taken when the session was created, so while either is still false we
// reconcile against the identity provider — this picks up email/phone verified
// elsewhere (Zitadel console, the email link, another device).
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	data, sid, err := h.sessions.FromRequest(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated", "not signed in")
		return
	}
	// The shared demo account is for products only — the onboarding portal behaves
	// as if no one is signed in (no account menu, "Try it now" stays available).
	if data.Demo {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated", "not signed in")
		return
	}

	emailVerified, phoneVerified := data.EmailVerified, data.PhoneVerified
	if !emailVerified || !phoneVerified {
		sub := data.ZitadelSub
		if sub == "" {
			sub = data.UserID
		}
		if u, e := h.users.Get(r.Context(), sub); e == nil {
			if u.EmailVerified != emailVerified || u.PhoneVerified != phoneVerified {
				emailVerified, phoneVerified = u.EmailVerified, u.PhoneVerified
				_ = h.sessions.Update(r.Context(), sid, func(d *session.Data) {
					d.EmailVerified = u.EmailVerified
					d.PhoneVerified = u.PhoneVerified
				})
			}
		}
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":            data.UserID,
		"email":         data.Email,
		"firstName":     data.FirstName,
		"lastName":      data.LastName,
		"displayName":   data.DisplayName,
		"phone":         data.Phone,
		"role":          data.Role,
		"emailVerified": emailVerified,
		"phoneVerified": phoneVerified,
	})
}

// Validate is the central session-validation endpoint products call (over HTTP, with
// the session cookie). It returns the UNIVERSAL identity — the Zitadel sub + email.
// Each product maps the sub to its own internal id / tenant / role on its side; this
// service is standalone and asserts nothing product-specific.
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	data, _, err := h.sessions.FromRequest(r)
	if err != nil {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]any{"valid": false})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"valid":          true,
		"sub":            data.ZitadelSub, // universal identity (the IdP subject)
		"email":          data.Email,
		"email_verified": data.EmailVerified,
		"first_name":     data.FirstName,
		"last_name":      data.LastName,
		"display_name":   data.DisplayName,
		"phone":          data.Phone,
	})
}

// Logout destroys the session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if _, id, err := h.sessions.FromRequest(r); err == nil {
		h.sessions.Delete(r.Context(), w, id)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// DemoLogin signs the visitor in as the shared demo account — "Try it now" with no
// signup. The demo user is a real account on the shared SSO session, so the demo
// carries across every product.
func (h *Handler) DemoLogin(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.DemoEnabled || h.cfg.DemoPassword == "" {
		httpx.WriteError(w, http.StatusNotFound, "demo_disabled", "Demo isn't available.")
		return
	}
	user, err := h.users.EnsureUser(r.Context(), identity.RegisterInput{
		FirstName:   h.cfg.DemoFirstName,
		LastName:    h.cfg.DemoLastName,
		DisplayName: "Demo",
		Email:       h.cfg.DemoEmail,
		Password:    h.cfg.DemoPassword,
		Company:     "Simplify",
		JobTitle:    "Demo",
	})
	if err != nil {
		h.log.Error("demo login: ensure user", zap.Error(err))
		httpx.WriteError(w, http.StatusBadGateway, "demo_unavailable", "Demo isn't available right now.")
		return
	}
	user.EmailVerified = true
	if err := h.establishSession(w, r, user, true, "demo"); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not start the demo session")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "next": "/", "demo": true})
}

// establishSession creates the central SSO session for an authenticated user.
//
// The session's identity is UNIVERSAL: the Zitadel sub. This service is standalone —
// it stores no product-specific ids, tenants or roles. Each product validates the
// session via /auth/validate and maps the sub to its own internal id on its side.
func (h *Handler) establishSession(w http.ResponseWriter, r *http.Request, user identity.User, demo bool, authMethod string) error {
	data := session.Data{
		UserID:        user.ID, // the universal identity = Zitadel sub
		ZitadelSub:    user.ID,
		Email:         user.Email,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		DisplayName:   user.DisplayName,
		Phone:         user.Phone,
		EmailVerified: user.EmailVerified,
		PhoneVerified: user.PhoneVerified,
		Demo:          demo,
	}

	if _, err := h.sessions.Create(r.Context(), w, data); err != nil {
		return err
	}

	// Persist to the onboarding DB (best-effort — never block sign-in on it).
	h.persistUser(r, user, demo, authMethod)
	return nil
}

// persistUser syncs the account into the onboarding DB and appends a login-audit row.
// Everything is keyed by the UNIVERSAL identity (user.ID == Zitadel sub) — no
// product-specific ids. Best-effort: failures are logged, not surfaced.
func (h *Handler) persistUser(r *http.Request, user identity.User, demo bool, authMethod string) {
	if h.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
	defer cancel()

	if err := h.db.UpsertUser(ctx, store.UserRecord{
		ZitadelSub:    user.ID,
		Email:         user.Email,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		DisplayName:   user.DisplayName,
		Phone:         user.Phone,
		Company:       user.Company,
		JobTitle:      user.JobTitle,
		AuthMethod:    authMethod,
		EmailVerified: user.EmailVerified,
		PhoneVerified: user.PhoneVerified,
		IsDemo:        demo,
	}); err != nil {
		h.log.Warn("persist user failed", zap.Error(err))
	}

	if err := h.db.RecordLogin(ctx, store.LoginAudit{
		UserID:     user.ID, // universal subject (Zitadel sub)
		Email:      user.Email,
		AuthMethod: authMethod,
		Success:    true,
		IP:         clientIP(r),
		UserAgent:  r.UserAgent(),
	}); err != nil {
		h.log.Warn("record login failed", zap.Error(err))
	}
}

// clientIP extracts the best-guess client IP (proxy-aware).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// debug returns the code only when DEBUG_RETURN_CODE is on (dev/testing).
func (h *Handler) debug(code string) string {
	if h.cfg.DebugReturnCode {
		return code
	}
	return ""
}
