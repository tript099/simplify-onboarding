// Package notify sends the demo/POC/contact emails (requester confirmation, team
// notification, meeting invite) through the shared MailForge service.
//
// Email HTML lives in MailForge as published template versions under the onboarding
// project — this service holds NO email HTML. It only builds the template variables
// and sends by template_id. (See scripts/provision-email-templates.py.)
//
// Sending is best-effort: callers fire it in a goroutine after persisting the request,
// so email never blocks or fails the user's submission.
package notify

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"github.com/simplify/onboarding/internal/mailforge"
	"go.uber.org/zap"
)

// KV is a label/value row in the email recap tables (rendered by the MailForge template
// via {{range .Items}}{{.Label}}/{{.Value}}{{end}}).
type KV struct {
	Label string `json:"Label"`
	Value string `json:"Value"`
}

// Request is the data a notification needs (mapped from a demo_requests row).
type Request struct {
	ID            string
	Type          string // demo | poc | contact
	ProductKey    string
	ProductName   string
	FirstName     string
	LastName      string
	Email         string
	Phone         string
	JobTitle      string
	Company       string
	CompanySize   string
	Industry      string
	Country       string
	UseCase       string
	Timeline      string
	Budget        string
	Message       string
	PreferredDate string
	TimeSlot      string
	Timezone      string
}

// Config configures the notifier. The Template* ids point at published MailForge
// template collections under the onboarding project.
type Config struct {
	FromName  string
	FromEmail string
	ReplyTo   string
	PortalURL string // builds the "Try it now" link in the confirmation

	ConfirmationTemplateID string
	TeamNotifyTemplateID   string
	InviteTemplateID       string
}

// Service builds variables + sends the demo/POC emails via MailForge templates.
type Service struct {
	mf         *mailforge.Client
	recipients *recipientsStore
	cfg        Config
	log        *zap.Logger
}

// New builds the notify service. recipientsFile is the JSON routing file.
func New(mf *mailforge.Client, recipientsFile string, cfg Config, log *zap.Logger) *Service {
	return &Service{
		mf:         mf,
		recipients: newRecipientsStore(recipientsFile),
		cfg:        cfg,
		log:        log,
	}
}

// Enabled reports whether emails can actually be sent.
func (s *Service) Enabled() bool { return s.mf.Enabled() }

// Recipients resolves the team addresses for a request type/product (for adding them
// as meeting attendees).
func (s *Service) Recipients(reqType, product string) []string {
	return s.recipients.resolve(reqType, product)
}

// SendForRequest sends BOTH the requester confirmation and the team notification.
func (s *Service) SendForRequest(ctx context.Context, r Request) (confirmationID, notifyID string) {
	if !s.mf.Enabled() {
		s.log.Warn("notify: mailforge not configured — skipping emails", zap.String("request_id", r.ID))
		return "", ""
	}
	return s.SendConfirmation(ctx, r), s.SendTeamNotify(ctx, r, "", "", time.Time{}, 0)
}

// SendConfirmation emails the requester their submission confirmation.
func (s *Service) SendConfirmation(ctx context.Context, r Request) string {
	if r.Email == "" {
		return ""
	}
	return s.send(ctx, "confirmation", s.cfg.ConfirmationTemplateID, mailforge.SendRequest{
		IdempotencyKey: "demo-confirm:" + r.ID,
		ReplyTo:        s.cfg.ReplyTo,
		To:             []string{r.Email},
		Variables: map[string]any{
			"FirstName":    orVal(r.FirstName, "there"),
			"ProductName":  orVal(r.ProductName, "Simplify"),
			"RequestLabel": requestLabel(r.Type),
			"ReplyWindow":  "24 business hours",
			"Items":        confirmationItems(r),
			"TryURL":       s.tryURL(r),
			"Year":         time.Now().Year(),
		},
		Tags: []string{"onboarding", "demo-confirmation", r.Type},
	}, r.ID)
}

// SendTeamNotify emails the routed Simplify team about a new request. When the meeting
// is already scheduled (meetingURL set), the same email carries the join link + an .ics
// so the team gets ONE organized email (and no separate raw scheduler invite).
func (s *Service) SendTeamNotify(ctx context.Context, r Request, meetingURL, whenLine string, startTime time.Time, durationMin int) string {
	to := s.recipients.resolve(r.Type, r.ProductKey)
	if len(to) == 0 {
		s.log.Warn("notify: no team recipients configured", zap.String("request_id", r.ID))
		return ""
	}
	req := mailforge.SendRequest{
		IdempotencyKey: "demo-notify:" + r.ID,
		ReplyTo:        r.Email, // reps reply straight to the requester
		To:             to,
		Variables: map[string]any{
			"RequestTypeLabel": strings.ToUpper(orVal(r.Type, "demo")),
			"ReceivedAt":       time.Now().Format("Mon 2 Jan, 15:04 MST"),
			"Company":          orVal(r.Company, "(no company)"),
			"ProductName":      r.ProductName,
			"ContactName":      strings.TrimSpace(r.FirstName + " " + r.LastName),
			"ContactEmail":     r.Email,
			"Phone":            r.Phone,
			"Items":            teamItems(r),
			"RequestID":        r.ID,
			"MeetingURL":       meetingURL,
			"WhenLine":         whenLine,
		},
		Tags: []string{"onboarding", "demo-team-notify", r.Type},
	}
	if meetingURL != "" {
		ics := buildICS(r, meetingURL, startTime, durationMin, s.cfg.FromEmail)
		req.Attachments = []mailforge.Attachment{{
			Filename:    "invite.ics",
			ContentType: "text/calendar; charset=utf-8; method=REQUEST",
			Content:     base64.StdEncoding.EncodeToString([]byte(ics)),
			Disposition: "attachment",
		}}
	}
	return s.send(ctx, "team-notify", s.cfg.TeamNotifyTemplateID, req, r.ID)
}

// SendInvite emails the requester the meeting invite with an .ics calendar attachment
// (the branded HTML is a MailForge template; the .ics is generated here as calendar
// data, not email HTML).
func (s *Service) SendInvite(ctx context.Context, r Request, meetingURL, whenLine string, startTime time.Time, durationMin int) string {
	if r.Email == "" || meetingURL == "" {
		return ""
	}
	ics := buildICS(r, meetingURL, startTime, durationMin, s.cfg.FromEmail)
	return s.send(ctx, "invite", s.cfg.InviteTemplateID, mailforge.SendRequest{
		IdempotencyKey: "demo-invite:" + r.ID,
		ReplyTo:        s.cfg.ReplyTo,
		To:             []string{r.Email},
		Variables: map[string]any{
			"FirstName":    orVal(r.FirstName, "there"),
			"ProductName":  r.ProductName,
			"RequestLabel": requestLabel(r.Type),
			"WhenLine":     whenLine,
			"MeetingURL":   meetingURL,
			"Year":         time.Now().Year(),
		},
		Attachments: []mailforge.Attachment{{
			Filename:    "invite.ics",
			ContentType: "text/calendar; charset=utf-8; method=REQUEST",
			Content:     base64.StdEncoding.EncodeToString([]byte(ics)),
			Disposition: "attachment",
		}},
		Tags: []string{"onboarding", "demo-invite", r.Type},
	}, r.ID)
}

// send fills in the From + TemplateID and dispatches. Returns the message id (or "").
func (s *Service) send(ctx context.Context, kind, templateID string, req mailforge.SendRequest, requestID string) string {
	if !s.mf.Enabled() {
		return ""
	}
	if templateID == "" {
		s.log.Warn("notify: template id not configured — skipping", zap.String("kind", kind), zap.String("request_id", requestID))
		return ""
	}
	req.From = mailforge.Address{Name: s.cfg.FromName, Email: s.cfg.FromEmail}
	req.TemplateID = templateID
	res, err := s.mf.Send(ctx, req)
	if err != nil {
		s.log.Error("notify: send failed", zap.String("kind", kind), zap.Error(err), zap.String("request_id", requestID))
		return ""
	}
	return res.ID
}

func (s *Service) tryURL(r Request) string {
	if s.cfg.PortalURL == "" || r.ProductKey == "" {
		return ""
	}
	return strings.TrimRight(s.cfg.PortalURL, "/") + "/product/" + r.ProductKey
}

// buildICS produces an iCalendar REQUEST for the meeting (adds it to the recipient's
// calendar with the join link as the location). This is calendar data, not email HTML.
func buildICS(r Request, meetingURL string, start time.Time, durationMin int, organizer string) string {
	if durationMin <= 0 {
		durationMin = 30
	}
	end := start.Add(time.Duration(durationMin) * time.Minute)
	stamp := func(t time.Time) string { return t.UTC().Format("20060102T150405Z") }
	esc := func(s string) string { // escape per RFC 5545
		s = strings.ReplaceAll(s, "\\", "\\\\")
		s = strings.ReplaceAll(s, ";", "\\;")
		s = strings.ReplaceAll(s, ",", "\\,")
		s = strings.ReplaceAll(s, "\n", "\\n")
		return s
	}
	name := strings.TrimSpace(r.FirstName + " " + r.LastName)
	summary := "Simplify " + titleCase(r.Type) + " — " + orVal(r.ProductName, "Simplify")
	desc := "Your " + requestLabel(r.Type) + " with the Simplify team.\nJoin: " + meetingURL
	lines := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//Simplify//Onboarding//EN",
		"CALSCALE:GREGORIAN",
		"METHOD:REQUEST",
		"BEGIN:VEVENT",
		"UID:" + r.ID + "@simplifyai.id",
		"DTSTAMP:" + stamp(time.Now()),
		"DTSTART:" + stamp(start),
		"DTEND:" + stamp(end),
		"SUMMARY:" + esc(summary),
		"DESCRIPTION:" + esc(desc),
		"LOCATION:" + esc(meetingURL),
		"ORGANIZER;CN=Simplify:mailto:" + organizer,
		"ATTENDEE;CN=" + esc(name) + ";RSVP=TRUE:mailto:" + r.Email,
		"STATUS:CONFIRMED",
		"SEQUENCE:0",
		"END:VEVENT",
		"END:VCALENDAR",
	}
	return strings.Join(lines, "\r\n")
}

// ── variable helpers ──────────────────────────────────────────────

func titleCase(t string) string {
	switch t {
	case "poc":
		return "POC"
	case "contact":
		return "call"
	default:
		return "Demo"
	}
}

func requestLabel(t string) string {
	switch strings.ToLower(t) {
	case "poc":
		return "proof of concept request"
	case "contact":
		return "message"
	default:
		return "demo request"
	}
}

func confirmationItems(r Request) []KV {
	items := []KV{}
	add := func(l, v string) {
		if strings.TrimSpace(v) != "" {
			items = append(items, KV{l, v})
		}
	}
	add("Product", r.ProductName)
	add("Use case", r.UseCase)
	add("Timeline", r.Timeline)
	add("Preferred time", preferredSlot(r))
	add("Message", r.Message)
	return items
}

func teamItems(r Request) []KV {
	items := []KV{}
	add := func(l, v string) {
		if strings.TrimSpace(v) != "" {
			items = append(items, KV{l, v})
		}
	}
	add("Job title", r.JobTitle)
	add("Company size", r.CompanySize)
	add("Industry", r.Industry)
	add("Country", r.Country)
	add("Use case", r.UseCase)
	add("Timeline", r.Timeline)
	add("Budget", r.Budget)
	add("Preferred time", preferredSlot(r))
	add("Message", r.Message)
	return items
}

func preferredSlot(r Request) string {
	parts := []string{}
	for _, p := range []string{r.PreferredDate, r.TimeSlot, r.Timezone} {
		if strings.TrimSpace(p) != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " · ")
}

func orVal(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}
