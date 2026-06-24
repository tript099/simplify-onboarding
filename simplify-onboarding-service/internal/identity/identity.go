// Package identity abstracts the user directory + credential/verification lifecycle
// behind a single interface. The production implementation is Zitadel (it stores
// passwords and sends verification emails/SMS); a self-contained "dev" provider
// exists only for offline development.
package identity

import (
	"context"
	"errors"
	"strings"
)

// Errors handlers map to user-facing copy / HTTP status codes.
var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrNotFound           = errors.New("user not found")
	ErrBadCode            = errors.New("incorrect verification code")
	ErrCodeExpired        = errors.New("verification code expired")
	ErrNotConfigured      = errors.New("identity provider not configured")
)

// RegisterInput is the data captured at the create-account form / value wall.
type RegisterInput struct {
	FirstName   string
	LastName    string
	DisplayName string
	Email       string
	Phone       string
	Company     string
	JobTitle    string
	Password    string
}

// User is the canonical identity record.
type User struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	FirstName     string `json:"firstName,omitempty"`
	LastName      string `json:"lastName,omitempty"`
	DisplayName   string `json:"displayName,omitempty"`
	Phone         string `json:"phone,omitempty"`
	Company       string `json:"company,omitempty"`
	JobTitle      string `json:"jobTitle,omitempty"`
	Role          string `json:"role,omitempty"`
	EmailVerified bool   `json:"emailVerified"`
	PhoneVerified bool   `json:"phoneVerified"`
}

// Provider is the user-directory + verification contract.
//
// Register creates the user AND triggers email verification — the provider (e.g.
// Zitadel) sends the code to the user's inbox. The returned debugCode is non-empty
// ONLY when the provider is configured to return codes for local testing (never in
// production); handlers must never expose it unless explicitly in debug mode.
type Provider interface {
	Register(ctx context.Context, in RegisterInput) (user User, debugCode string, err error)

	VerifyEmail(ctx context.Context, userID, code string) error
	ResendEmail(ctx context.Context, userID string) (debugCode string, err error)

	StartPhoneVerification(ctx context.Context, userID string) (debugCode string, err error)
	VerifyPhone(ctx context.Context, userID, code string) error

	Authenticate(ctx context.Context, email, password string) (User, error)
	Get(ctx context.Context, userID string) (User, error)
}

// NormalizeEmail lower-cases and trims an email for consistent indexing.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
