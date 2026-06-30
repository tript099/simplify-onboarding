package store

import (
	"context"
	"fmt"
)

// UserRecord is the onboarding profile persisted on every sign-up / sign-in.
// Keyed by the universal identity (ZitadelSub) — no per-product ids live here.
type UserRecord struct {
	ZitadelSub    string
	Email         string
	FirstName     string
	LastName      string
	DisplayName   string
	Phone         string
	Company       string
	JobTitle      string
	AuthMethod    string // password | otp | google | microsoft | demo
	EmailVerified bool
	PhoneVerified bool
	IsDemo        bool
}

// UpsertUser inserts or updates the user keyed by zitadel_sub, and bumps the login
// counter + last_login_at. Non-empty incoming fields win; empty ones keep the stored
// value (so a lighter login doesn't wipe a fuller registration profile).
func (s *Store) UpsertUser(ctx context.Context, u UserRecord) error {
	const q = `
INSERT INTO users (
    zitadel_sub, email, first_name, last_name, display_name,
    phone, company, job_title, auth_method, email_verified, phone_verified,
    is_demo, login_count, last_login_at
) VALUES (
    $1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''),
    NULLIF($6,''), NULLIF($7,''), NULLIF($8,''), NULLIF($9,''), $10, $11,
    $12, 1, now()
)
ON CONFLICT (zitadel_sub) DO UPDATE SET
    email          = EXCLUDED.email,
    first_name     = COALESCE(NULLIF(EXCLUDED.first_name,''),   users.first_name),
    last_name      = COALESCE(NULLIF(EXCLUDED.last_name,''),    users.last_name),
    display_name   = COALESCE(NULLIF(EXCLUDED.display_name,''), users.display_name),
    phone          = COALESCE(NULLIF(EXCLUDED.phone,''),        users.phone),
    company        = COALESCE(NULLIF(EXCLUDED.company,''),      users.company),
    job_title      = COALESCE(NULLIF(EXCLUDED.job_title,''),    users.job_title),
    auth_method    = COALESCE(NULLIF(EXCLUDED.auth_method,''),  users.auth_method),
    email_verified = users.email_verified OR EXCLUDED.email_verified,
    phone_verified = users.phone_verified OR EXCLUDED.phone_verified,
    is_demo        = EXCLUDED.is_demo,
    login_count    = users.login_count + 1,
    last_login_at  = now(),
    updated_at     = now()`
	_, err := s.pool.Exec(ctx, q,
		u.ZitadelSub, u.Email, u.FirstName, u.LastName, u.DisplayName,
		u.Phone, u.Company, u.JobTitle, u.AuthMethod, u.EmailVerified, u.PhoneVerified,
		u.IsDemo,
	)
	if err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}
	return nil
}
