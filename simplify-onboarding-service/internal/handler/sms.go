package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/simplify/onboarding/internal/httpx"
	"github.com/simplify/onboarding/internal/sms"
	"go.uber.org/zap"
)

// zitadelSMSPayload is the subset of Zitadel's HTTP SMS provider webhook body we use.
// (contextInfo.recipientPhoneNumber + the rendered text in templateData.text; args.code
// is the raw OTP as a fallback.)
type zitadelSMSPayload struct {
	ContextInfo struct {
		RecipientPhoneNumber string `json:"recipientPhoneNumber"`
		EventType            string `json:"eventType"`
	} `json:"contextInfo"`
	TemplateData struct {
		Text string `json:"text"`
	} `json:"templateData"`
	Args struct {
		Code string `json:"code"`
	} `json:"args"`
}

// ZitadelSMS is the webhook Zitadel's HTTP SMS provider POSTs OTP messages to. It verifies
// the ZITADEL-Signature, then forwards the phone + text to the configured SMS gateway.
// POST /sms/zitadel
func (h *Handler) ZitadelSMS(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "could not read body")
		return
	}

	// Verify the signature (skip only if explicitly disabled or no key set — for first-run testing).
	if h.cfg.SMSWebhookVerify && h.cfg.SMSWebhookSigningKey != "" {
		if verr := sms.VerifySignature(h.cfg.SMSWebhookSigningKey, r.Header.Get("ZITADEL-Signature"), body); verr != nil {
			h.log.Warn("zitadel sms webhook: signature verification failed", zap.Error(verr))
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_signature", "signature verification failed")
			return
		}
	} else if h.cfg.SMSWebhookSigningKey == "" {
		h.log.Warn("zitadel sms webhook: SMS_WEBHOOK_SIGNING_KEY not set — accepting unverified request")
	}

	var msg zitadelSMSPayload
	if err := json.Unmarshal(body, &msg); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid payload")
		return
	}
	phone := msg.ContextInfo.RecipientPhoneNumber
	text := msg.TemplateData.Text
	if text == "" && msg.Args.Code != "" {
		text = "Your Simplify verification code is " + msg.Args.Code
	}
	if phone == "" || text == "" {
		h.log.Warn("zitadel sms webhook: missing phone or text", zap.String("event", msg.ContextInfo.EventType))
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "missing recipient or message")
		return
	}

	attempted, err := h.deliverSMS(r.Context(), phone, text)
	if !attempted {
		// No transport configured → log-only (lets you wire Zitadel + test the round-trip first).
		h.log.Warn("zitadel sms webhook: no SMS transport configured — logging OTP instead of sending",
			zap.String("provider", h.cfg.SMSProvider), zap.String("phone", phone), zap.String("text", text))
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if err != nil {
		h.log.Error("zitadel sms webhook: SMS delivery failed", zap.Error(err), zap.String("phone", phone))
		httpx.WriteError(w, http.StatusBadGateway, "sms_send_failed", "could not deliver SMS")
		return
	}
	h.log.Info("zitadel sms webhook: OTP delivered", zap.String("provider", h.cfg.SMSProvider), zap.String("phone", phone), zap.String("event", msg.ContextInfo.EventType))
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// deliverSMS routes the OTP to the configured transport. Returns attempted=false when the
// chosen transport isn't configured (→ caller logs the OTP instead, for testing).
func (h *Handler) deliverSMS(ctx context.Context, phone, text string) (attempted bool, err error) {
	switch strings.ToLower(strings.TrimSpace(h.cfg.SMSProvider)) {
	case "sns":
		if h.cfg.SMSSNSRegion == "" {
			return false, nil
		}
		return true, sms.SendSNS(ctx, sms.SNSConfig{
			Region:   h.cfg.SMSSNSRegion,
			SenderID: h.cfg.SMSSNSSenderID,
			SMSType:  h.cfg.SMSSNSType,
		}, phone, text)
	default: // "http"
		if h.cfg.SMSProviderURL == "" {
			return false, nil
		}
		return true, sms.Forward(ctx, sms.Config{
			URL:          h.cfg.SMSProviderURL,
			Method:       h.cfg.SMSProviderMethod,
			ContentType:  h.cfg.SMSProviderContentType,
			AuthHeader:   h.cfg.SMSProviderAuthHeader,
			BodyTemplate: h.cfg.SMSProviderBodyTemplate,
		}, phone, text)
	}
}
