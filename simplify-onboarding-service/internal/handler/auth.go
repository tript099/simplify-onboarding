package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/simplify/onboarding/internal/httpx"
	"github.com/simplify/onboarding/internal/identity"
	"github.com/simplify/onboarding/internal/session"
	"github.com/simplify/onboarding/internal/userstore"
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

	if err := h.establishSession(w, r, user); err != nil {
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
	if err := h.establishSession(w, r, user); err != nil {
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

// Validate is the session-validation endpoint products / Kong call (mirrors the
// docflow-auth `/validate` contract). It reads the central session cookie and
// returns the user + tenant + role context.
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	data, _, err := h.sessions.FromRequest(r)
	if err != nil {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]any{"valid": false})
		return
	}
	out := map[string]any{
		"valid":          true,
		"user_id":        data.UserID,
		"tenant_id":      data.TenantID,
		"role":           data.Role,
		"email":          data.Email,
		"email_verified": data.EmailVerified,
	}
	for k, v := range data.Attrs {
		out[k] = v // role_tier, custom_role_id, custom_role_name, is_org_member
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// Logout destroys the session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if _, id, err := h.sessions.FromRequest(r); err == nil {
		h.sessions.Delete(r.Context(), w, id)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// SSOStart begins a federated SSO flow (stub → Zitadel IdP intent).
func (h *Handler) SSOStart(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, h.cfg.FrontendURL, http.StatusFound)
}

// establishSession creates the central SSO session for an authenticated user.
//
// When the shared platform DB is wired (h.dir != nil), the session is written in
// the DocFlow schema — internal app_users.id as UserID, plus tenant + role attrs —
// so products reading the shared Redis pick it up directly (no second login).
func (h *Handler) establishSession(w http.ResponseWriter, r *http.Request, user identity.User) error {
	data := session.Data{
		UserID:        user.ID, // overwritten with internal id below when DB is present
		ZitadelSub:    user.ID,
		Email:         user.Email,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		DisplayName:   user.DisplayName,
		Phone:         user.Phone,
		Role:          user.Role,
		EmailVerified: user.EmailVerified,
		PhoneVerified: user.PhoneVerified,
	}

	if h.dir != nil {
		internalID, storedRole, err := h.dir.ResolveOrCreate(
			r.Context(), user.ID, user.Email, user.FirstName, user.LastName, user.Role,
			userstore.Profile{NickName: user.DisplayName, Phone: user.Phone},
		)
		if err != nil {
			h.log.Error("session: resolve user in shared directory", zap.Error(err))
			return err
		}
		tc := h.dir.ResolveTenantContext(r.Context(), internalID)
		data.UserID = internalID
		data.TenantID = tc.TenantID
		if storedRole != "" {
			data.Role = storedRole
		}
		data.Attrs = map[string]any{
			"role_tier":        tc.RoleTier,
			"custom_role_id":   tc.CustomRoleID,
			"custom_role_name": tc.CustomRoleName,
			"is_org_member":    tc.IsMember,
		}
	}

	_, err := h.sessions.Create(r.Context(), w, data)
	return err
}

// debug returns the code only when DEBUG_RETURN_CODE is on (dev/testing).
func (h *Handler) debug(code string) string {
	if h.cfg.DebugReturnCode {
		return code
	}
	return ""
}
