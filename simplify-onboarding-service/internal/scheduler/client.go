// Package scheduler is a client for the external meeting-scheduling API
// (scheduler.simplifyaipro.com). It creates a Google Meet / Teams meeting and
// returns the join URL. POST {path} with {attendees, start_time, duration, passcode}
// → { "meeting_url": "..." }.
package scheduler

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

// Client calls the scheduling API.
type Client struct {
	baseURL  string
	path     string
	passcode string
	duration int
	http     *http.Client
}

// New builds the client. Disabled (Enabled()==false) when URL/passcode are empty.
func New(baseURL, path, passcode string, durationMin int) *Client {
	if path == "" {
		path = "/schedule_google"
	}
	if durationMin <= 0 {
		durationMin = 30
	}
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		path:     "/" + strings.TrimLeft(path, "/"),
		passcode: passcode,
		duration: durationMin,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// Enabled reports whether the scheduler is configured.
func (c *Client) Enabled() bool {
	return c != nil && c.baseURL != "" && c.passcode != ""
}

// DefaultDuration is the configured meeting length in minutes.
func (c *Client) DefaultDuration() int { return c.duration }

// scheduleRequest is the API payload.
type scheduleRequest struct {
	Attendees   []string `json:"attendees"`
	StartTime   string   `json:"start_time"` // ISO-8601 WITHOUT trailing Z (API rejects Z)
	Duration    int      `json:"duration"`   // minutes
	Passcode    string   `json:"passcode"`
	Summary     string   `json:"summary,omitempty"`
	Description string   `json:"description,omitempty"`
}

// Meeting is the result of scheduling.
type Meeting struct {
	URL       string
	StartTime time.Time
	Duration  int
}

// Schedule creates a meeting and returns its join URL.
func (c *Client) Schedule(ctx context.Context, attendees []string, startTime time.Time, durationMin int, summary, description string) (*Meeting, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("scheduler not configured")
	}
	if durationMin <= 0 {
		durationMin = c.duration
	}
	payload := scheduleRequest{
		Attendees: attendees,
		// The API errors on a trailing "Z"; send naive ISO-8601 (UTC, no zone marker).
		StartTime:   startTime.UTC().Format("2006-01-02T15:04:05"),
		Duration:    durationMin,
		Passcode:    c.passcode,
		Summary:     summary,
		Description: description,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+c.path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call scheduler: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scheduler %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var out struct {
		MeetingURL string `json:"meeting_url"`
	}
	if json.Unmarshal(raw, &out) != nil || out.MeetingURL == "" {
		return nil, fmt.Errorf("scheduler: no meeting_url in response: %s", truncate(string(raw), 200))
	}
	return &Meeting{URL: out.MeetingURL, StartTime: startTime, Duration: durationMin}, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
