package store

import (
	"context"
	"encoding/json"
)

// InsertEvent records a funnel/analytics event. Best-effort by convention — callers
// should not fail the request if this errors.
func (s *Store) InsertEvent(ctx context.Context, visitorID, userID, event, productKey string, metadata map[string]any) error {
	var meta []byte
	if len(metadata) > 0 {
		meta, _ = json.Marshal(metadata)
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO onboarding_events (visitor_id, user_id, event, product_key, metadata)
		 VALUES (NULLIF($1,''), NULLIF($2,''), $3, NULLIF($4,''), $5)`,
		visitorID, userID, event, productKey, meta)
	return err
}

// LoginAudit is one authentication attempt.
type LoginAudit struct {
	UserID     string
	Email      string
	AuthMethod string
	Success    bool
	IP         string
	UserAgent  string
}

// RecordLogin appends to the auth audit trail.
func (s *Store) RecordLogin(ctx context.Context, a LoginAudit) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO login_audit (user_id, email, auth_method, success, ip, user_agent)
		 VALUES (NULLIF($1,''), NULLIF($2,''), NULLIF($3,''), $4, NULLIF($5,''), NULLIF($6,''))`,
		a.UserID, a.Email, a.AuthMethod, a.Success, a.IP, a.UserAgent)
	return err
}
