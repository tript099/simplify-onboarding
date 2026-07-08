// Package sms bridges Zitadel's HTTP SMS provider to any SMS gateway. Zitadel POSTs the
// OTP to our webhook (signed with ZITADEL-Signature); we verify it and forward the phone
// number + message to a downstream gateway configured entirely via env — so you can use
// ANY provider (not just Twilio) without code changes.
package sms

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config describes how to forward to your SMS gateway (all from SMS_PROVIDER_* env).
type Config struct {
	URL         string
	Method      string
	ContentType string
	AuthHeader  string // raw "Name: value"
	BodyTemplate string // uses {{phone}} and {{text}}
}

var httpClient = &http.Client{Timeout: 12 * time.Second}

// VerifySignature checks Zitadel's `ZITADEL-Signature: t=<unix>,v1=<hex>` header. The
// signed content is "<t>.<rawBody>", HMAC-SHA256 with the provider's signingKey (the
// Stripe-style scheme Zitadel uses). Returns nil when valid.
func VerifySignature(signingKey, header string, body []byte) error {
	if signingKey == "" {
		return errors.New("no signing key configured")
	}
	var t, v1 string
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			t = kv[1]
		case "v1":
			v1 = kv[1]
		}
	}
	if t == "" || v1 == "" {
		return fmt.Errorf("malformed ZITADEL-Signature header %q", header)
	}
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(t + "." + string(body)))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(v1)) {
		return errors.New("signature mismatch")
	}
	return nil
}

// Forward renders the configured body template with the phone + text and POSTs it to the
// downstream gateway. Returns an error on transport failure or a non-2xx response.
func Forward(ctx context.Context, c Config, phone, text string) error {
	method := c.Method
	if method == "" {
		method = http.MethodPost
	}
	body := render(c.BodyTemplate, c.ContentType, phone, text)
	req, err := http.NewRequestWithContext(ctx, method, c.URL, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("build sms request: %w", err)
	}
	if c.ContentType != "" {
		req.Header.Set("Content-Type", c.ContentType)
	}
	if c.AuthHeader != "" {
		if name, val, ok := strings.Cut(c.AuthHeader, ":"); ok {
			req.Header.Set(strings.TrimSpace(name), strings.TrimSpace(val))
		}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sms gateway request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("sms gateway status %d: %s", resp.StatusCode, string(snippet))
	}
	return nil
}

// render substitutes {{phone}}/{{text}}, escaping them for the target content type so the
// body stays valid (JSON-escape for JSON, URL-escape for form-encoded).
func render(tmpl, contentType, phone, text string) string {
	p, t := phone, text
	switch {
	case strings.Contains(contentType, "json"):
		p, t = jsonEscape(phone), jsonEscape(text)
	case strings.Contains(contentType, "form"):
		p, t = url.QueryEscape(phone), url.QueryEscape(text)
	}
	return strings.NewReplacer("{{phone}}", p, "{{text}}", t).Replace(tmpl)
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1]) // strip the surrounding quotes
}
