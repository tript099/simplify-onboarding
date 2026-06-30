package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// DemoRequest is a sales-led submission (demo / POC / contact).
type DemoRequest struct {
	ID            string
	Type          string
	ProductKey    string
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
	Notes         string
	Message       string
	PreferredDate string
	TimeSlot      string
	Timezone      string
	Consent       bool
	Payload       map[string]any // full raw submission
}

// InsertDemoRequest persists a sales request and returns its generated id.
func (s *Store) InsertDemoRequest(ctx context.Context, d DemoRequest) (string, error) {
	payload, _ := json.Marshal(d.Payload)
	const q = `
INSERT INTO demo_requests (
    type, product_key, first_name, last_name, email, phone, job_title, company,
    company_size, industry, country, use_case, timeline, budget, notes, message,
    preferred_date, time_slot, timezone, consent, payload
) VALUES (
    $1, NULLIF($2,''), NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), NULLIF($6,''),
    NULLIF($7,''), NULLIF($8,''), NULLIF($9,''), NULLIF($10,''), NULLIF($11,''),
    NULLIF($12,''), NULLIF($13,''), NULLIF($14,''), NULLIF($15,''), NULLIF($16,''),
    NULLIF($17,''), NULLIF($18,''), NULLIF($19,''), $20, $21
)
RETURNING id`
	var id string
	err := s.pool.QueryRow(ctx, q,
		nz(d.Type, "demo"), d.ProductKey, d.FirstName, d.LastName, d.Email, d.Phone,
		d.JobTitle, d.Company, d.CompanySize, d.Industry, d.Country, d.UseCase,
		d.Timeline, d.Budget, d.Notes, d.Message, d.PreferredDate, d.TimeSlot,
		d.Timezone, d.Consent, payload,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert demo request: %w", err)
	}
	return id, nil
}

// GetDemoRequest loads the fields needed to schedule + email a meeting.
func (s *Store) GetDemoRequest(ctx context.Context, id string) (DemoRequest, error) {
	var d DemoRequest
	d.ID = id
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(type,''), COALESCE(product_key,''), COALESCE(first_name,''),
		        COALESCE(last_name,''), COALESCE(email,''), COALESCE(company,'')
		   FROM demo_requests WHERE id = $1`, id).
		Scan(&d.Type, &d.ProductKey, &d.FirstName, &d.LastName, &d.Email, &d.Company)
	if err != nil {
		return DemoRequest{}, fmt.Errorf("get demo request: %w", err)
	}
	return d, nil
}

// UpdateDemoScheduled stores the meeting URL + confirmed time and marks 'scheduled'.
func (s *Store) UpdateDemoScheduled(ctx context.Context, id, meetingURL string, scheduledAt time.Time, inviteEmailID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE demo_requests
		    SET meeting_url     = $2,
		        scheduled_at    = $3,
		        invite_email_id = COALESCE(NULLIF($4,''), invite_email_id),
		        status          = 'scheduled',
		        updated_at      = now()
		  WHERE id = $1`,
		id, meetingURL, scheduledAt, inviteEmailID)
	if err != nil {
		return fmt.Errorf("update demo scheduled: %w", err)
	}
	return nil
}

// UpdateDemoEmails records the MailForge message ids and marks the request notified.
func (s *Store) UpdateDemoEmails(ctx context.Context, id, confirmationID, notifyID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE demo_requests
		   SET confirmation_email_id = COALESCE(NULLIF($2,''), confirmation_email_id),
		       notify_email_id       = COALESCE(NULLIF($3,''), notify_email_id),
		       status                = CASE WHEN status = 'new' THEN 'notified' ELSE status END,
		       updated_at            = now()
		 WHERE id = $1`,
		id, confirmationID, notifyID)
	if err != nil {
		return fmt.Errorf("update demo emails: %w", err)
	}
	return nil
}

func nz(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
