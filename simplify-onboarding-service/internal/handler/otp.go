package handler

import (
	"errors"
	"net/http"

	"github.com/simplify/onboarding/internal/httpx"
	"github.com/simplify/onboarding/internal/identity"
	"github.com/simplify/onboarding/internal/session"
	"go.uber.org/zap"
)

type resendRequest struct {
	VerificationID string `json:"verificationId"`
}

type resendResponse struct {
	ResendIn  int    `json:"resendIn"`
	DebugCode string `json:"debugCode,omitempty"`
}

type verifyRequest struct {
	VerificationID string `json:"verificationId"`
	Code           string `json:"code"`
}

type verifyResponse struct {
	Verified bool   `json:"verified"`
	Next     string `json:"next"`
}

// ResendEmailCode asks the identity provider to re-send the email code.
func (h *Handler) ResendEmailCode(w http.ResponseWriter, r *http.Request) {
	var req resendRequest
	if err := httpx.DecodeJSON(r, &req); err != nil || req.VerificationID == "" {
		bad(w, "verification id is required")
		return
	}
	code, err := h.users.ResendEmail(r.Context(), req.VerificationID)
	if err != nil {
		h.log.Error("resend email failed", zap.Error(err))
		httpx.WriteError(w, http.StatusBadGateway, "otp_send_failed", "could not resend your code")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resendResponse{ResendIn: h.cfg.ResendCooldownSeconds, DebugCode: h.debug(code)})
}

// VerifyEmailCode confirms the email code and flips the session's email_verified flag.
func (h *Handler) VerifyEmailCode(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := httpx.DecodeJSON(r, &req); err != nil || req.VerificationID == "" {
		bad(w, "verification id and code are required")
		return
	}
	err := h.users.VerifyEmail(r.Context(), req.VerificationID, req.Code)
	if h.writeVerifyError(w, err) {
		return
	}
	// Reflect verification in the active session (if any).
	if _, sid, e := h.sessions.FromRequest(r); e == nil {
		_ = h.sessions.Update(r.Context(), sid, func(d *session.Data) { d.EmailVerified = true })
	}
	httpx.WriteJSON(w, http.StatusOK, verifyResponse{Verified: true, Next: "/"})
}

// StartMobileCode sends an SMS verification code to the signed-in user (deferred).
func (h *Handler) StartMobileCode(w http.ResponseWriter, r *http.Request) {
	data, _, err := h.sessions.FromRequest(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated", "sign in first")
		return
	}
	code, err := h.users.StartPhoneVerification(r.Context(), data.UserID)
	if err != nil {
		h.log.Error("start phone verify failed", zap.Error(err))
		httpx.WriteError(w, http.StatusBadGateway, "otp_send_failed", "could not send your verification code")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resendResponse{ResendIn: h.cfg.ResendCooldownSeconds, DebugCode: h.debug(code)})
}

// VerifyMobileCode confirms a mobile code and flips the session's phone_verified flag.
func (h *Handler) VerifyMobileCode(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := httpx.DecodeJSON(r, &req); err != nil || req.VerificationID == "" {
		bad(w, "verification id and code are required")
		return
	}
	err := h.users.VerifyPhone(r.Context(), req.VerificationID, req.Code)
	if h.writeVerifyError(w, err) {
		return
	}
	if _, sid, e := h.sessions.FromRequest(r); e == nil {
		_ = h.sessions.Update(r.Context(), sid, func(d *session.Data) { d.PhoneVerified = true })
	}
	httpx.WriteJSON(w, http.StatusOK, verifyResponse{Verified: true, Next: "/"})
}

// writeVerifyError maps verification errors to responses. Returns true if it wrote one.
func (h *Handler) writeVerifyError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, identity.ErrBadCode):
		httpx.WriteError(w, http.StatusBadRequest, "bad_otp", "That code isn't right. Check it and try again.")
	case errors.Is(err, identity.ErrCodeExpired):
		httpx.WriteError(w, http.StatusBadRequest, "otp_expired", "That code has expired. Request a new one.")
	default:
		h.log.Error("verify failed", zap.Error(err))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not verify your code")
	}
	return true
}
