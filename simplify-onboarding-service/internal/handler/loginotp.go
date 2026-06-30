package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/simplify/onboarding/internal/httpx"
	"github.com/simplify/onboarding/internal/identity"
	"go.uber.org/zap"
)

// Passwordless sign-in. The identity provider issues and delivers the one-time
// code (Zitadel-native: it emails/SMSes the code and tracks the OTP factor; dev:
// logs it). This handler only orchestrates and persists the opaque handle.

func loginOTPKey(id string) string { return "login_otp:" + id }

type startLoginOTPRequest struct {
	Identifier string `json:"identifier"` // email or phone
}

// StartLoginOTP looks up the user and triggers a login code.
func (h *Handler) StartLoginOTP(w http.ResponseWriter, r *http.Request) {
	var req startLoginOTPRequest
	if err := httpx.DecodeJSON(r, &req); err != nil || strings.TrimSpace(req.Identifier) == "" {
		bad(w, "enter your email or mobile number")
		return
	}

	handle, debugCode, err := h.users.StartLoginOTP(r.Context(), strings.TrimSpace(req.Identifier))
	if errors.Is(err, identity.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "no_account", "No account found for that email or mobile.")
		return
	}
	if err != nil {
		h.log.Error("start login otp failed", zap.Error(err))
		httpx.WriteError(w, http.StatusBadGateway, "otp_send_failed", "Couldn't send a sign-in code. Please try again.")
		return
	}

	id := uuid.NewString()
	raw, _ := json.Marshal(handle)
	if err := h.rdb.SetEx(r.Context(), loginOTPKey(id), raw, h.cfg.CodeTTL).Err(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not start sign-in")
		return
	}

	resp := map[string]any{"verificationId": id, "channel": handle.Channel, "resendIn": h.cfg.ResendCooldownSeconds}
	if h.cfg.DebugReturnCode && debugCode != "" {
		resp["debugCode"] = debugCode
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// VerifyLoginOTP exchanges a valid login code for a session.
func (h *Handler) VerifyLoginOTP(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := httpx.DecodeJSON(r, &req); err != nil || req.VerificationID == "" {
		bad(w, "verification id and code are required")
		return
	}

	raw, err := h.rdb.Get(r.Context(), loginOTPKey(req.VerificationID)).Bytes()
	if errors.Is(err, redis.Nil) {
		httpx.WriteError(w, http.StatusBadRequest, "otp_expired", "That code has expired. Request a new one.")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not verify your code")
		return
	}
	var handle identity.LoginOTPHandle
	if err := json.Unmarshal(raw, &handle); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not verify your code")
		return
	}

	user, err := h.users.VerifyLoginOTP(r.Context(), handle, req.Code)
	if errors.Is(err, identity.ErrBadCode) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_otp", "That code isn't right. Check it and try again.")
		return
	}
	if errors.Is(err, identity.ErrCodeExpired) {
		httpx.WriteError(w, http.StatusBadRequest, "otp_expired", "That code has expired. Request a new one.")
		return
	}
	if err != nil {
		h.log.Error("verify login otp failed", zap.Error(err))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not sign you in")
		return
	}

	// Consume the challenge and sign in.
	_ = h.rdb.Del(r.Context(), loginOTPKey(req.VerificationID)).Err()
	if err := h.establishSession(w, r, user, false, "otp"); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not start your session")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, verifyResponse{Verified: true, Next: "/"})
}
