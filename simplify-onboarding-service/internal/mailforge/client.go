// Package mailforge is a minimal client for the shared MailForge email service —
// the same service DocFlow uses. It sends a single email via POST /v1/send with a
// project-scoped Bearer API key. We render HTML ourselves and send it inline, so
// only the `email:send` scope is required (no template provisioning).
package mailforge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Address is a from/to identity.
type Address struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email"`
}

// Attachment is a file attached to an email (content is base64-encoded).
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Content     string `json:"content"` // base64
	Disposition string `json:"disposition"` // "attachment" | "inline"
}

// SendRequest is the /v1/send payload (subset we use). When TemplateID is set, the
// subject/html/text are rendered by MailForge from the published template version, and
// any inline Subject/HTML/Text here are ignored.
type SendRequest struct {
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	From           Address        `json:"from"`
	ReplyTo        string         `json:"reply_to,omitempty"`
	To             []string       `json:"to"`
	TemplateID     string         `json:"template_id,omitempty"`
	Variables      map[string]any `json:"variables,omitempty"`
	Subject        string         `json:"subject,omitempty"`
	HTML           string         `json:"html,omitempty"`
	Text           string         `json:"text,omitempty"`
	Attachments    []Attachment   `json:"attachments,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
}

// SendResult is the returned message id.
type SendResult struct {
	ID             string `json:"id"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// Client talks to the MailForge HTTP API.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// New builds a client. Send is a no-op error if baseURL/apiKey are empty (Enabled()).
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  strings.TrimSpace(apiKey),
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Enabled reports whether the client is configured to send.
func (c *Client) Enabled() bool {
	return c != nil && c.baseURL != "" && c.apiKey != ""
}

// Send delivers one email. Returns the MailForge message id.
func (c *Client) Send(ctx context.Context, req SendRequest) (*SendResult, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("mailforge not configured")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal send: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/send", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call mailforge: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= http.StatusBadRequest {
		msg := strings.TrimSpace(string(raw))
		var p struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &p) == nil && p.Error != "" {
			msg = p.Error
		}
		return nil, fmt.Errorf("mailforge %d: %s", resp.StatusCode, msg)
	}

	// Response is wrapped: { "data": { "id": "...", ... } }.
	var env struct {
		Data SendResult `json:"data"`
	}
	if json.Unmarshal(raw, &env) == nil && env.Data.ID != "" {
		return &env.Data, nil
	}
	var direct SendResult
	_ = json.Unmarshal(raw, &direct)
	return &direct, nil
}
