package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// DevProvider is a self-contained identity backend (Redis + bcrypt) for offline
// development ONLY. It generates real verification codes, logs them (so you can
// "receive" them without SMTP), and returns them as the debug code. Production
// uses the Zitadel provider instead.
type DevProvider struct {
	rdb       *redis.Client
	codeTTL   time.Duration
	returnDbg bool
	log       *zap.Logger
}

type record struct {
	User
	PasswordHash string `json:"passwordHash"`
}

// NewDevProvider builds the Redis-backed dev provider.
func NewDevProvider(rdb *redis.Client, codeTTL time.Duration, returnDebug bool, log *zap.Logger) *DevProvider {
	return &DevProvider{rdb: rdb, codeTTL: codeTTL, returnDbg: returnDebug, log: log}
}

func userKey(id string) string                 { return "user:" + id }
func emailKey(email string) string             { return "user_email:" + email }
func codeKey(channel, userID string) string    { return fmt.Sprintf("devcode:%s:%s", channel, userID) }

func (p *DevProvider) Register(ctx context.Context, in RegisterInput) (User, string, error) {
	email := NormalizeEmail(in.Email)
	id := uuid.NewString()

	ok, err := p.rdb.SetNX(ctx, emailKey(email), id, 0).Result()
	if err != nil {
		return User{}, "", fmt.Errorf("reserve email: %w", err)
	}
	if !ok {
		return User{}, "", ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		_ = p.rdb.Del(ctx, emailKey(email)).Err()
		return User{}, "", fmt.Errorf("hash password: %w", err)
	}

	rec := record{
		User: User{
			ID: id, Email: email, FirstName: in.FirstName, LastName: in.LastName,
			DisplayName: in.DisplayName, Phone: in.Phone, Company: in.Company,
			JobTitle: in.JobTitle, Role: "member",
		},
		PasswordHash: string(hash),
	}
	if err := p.save(ctx, rec); err != nil {
		_ = p.rdb.Del(ctx, emailKey(email)).Err()
		return User{}, "", err
	}

	code, err := p.issueCode(ctx, "email", id)
	if err != nil {
		return User{}, "", err
	}
	p.log.Info("dev user registered", zap.String("user_id", id), zap.String("email", email))
	return rec.User, p.maybe(code), nil
}

func (p *DevProvider) VerifyEmail(ctx context.Context, userID, code string) error {
	if err := p.checkCode(ctx, "email", userID, code); err != nil {
		return err
	}
	return p.mutate(ctx, userID, func(r *record) { r.EmailVerified = true })
}

func (p *DevProvider) ResendEmail(ctx context.Context, userID string) (string, error) {
	code, err := p.issueCode(ctx, "email", userID)
	if err != nil {
		return "", err
	}
	return p.maybe(code), nil
}

func (p *DevProvider) StartPhoneVerification(ctx context.Context, userID string) (string, error) {
	u, err := p.Get(ctx, userID)
	if err != nil {
		return "", err
	}
	if u.Phone == "" {
		return "", fmt.Errorf("no phone on file")
	}
	code, err := p.issueCode(ctx, "phone", userID)
	if err != nil {
		return "", err
	}
	return p.maybe(code), nil
}

func (p *DevProvider) VerifyPhone(ctx context.Context, userID, code string) error {
	if err := p.checkCode(ctx, "phone", userID, code); err != nil {
		return err
	}
	return p.mutate(ctx, userID, func(r *record) { r.PhoneVerified = true })
}

func (p *DevProvider) Authenticate(ctx context.Context, email, password string) (User, error) {
	id, err := p.rdb.Get(ctx, emailKey(NormalizeEmail(email))).Result()
	if errors.Is(err, redis.Nil) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("lookup email: %w", err)
	}
	rec, err := p.load(ctx, id)
	if err != nil {
		return User{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(rec.PasswordHash), []byte(password)) != nil {
		return User{}, ErrInvalidCredentials
	}
	return rec.User, nil
}

func (p *DevProvider) Get(ctx context.Context, id string) (User, error) {
	rec, err := p.load(ctx, id)
	if err != nil {
		return User{}, err
	}
	return rec.User, nil
}

// LookupByLogin (dev) resolves by email only — phone isn't indexed in dev.
func (p *DevProvider) LookupByLogin(ctx context.Context, identifier string) (User, error) {
	id, err := p.rdb.Get(ctx, emailKey(NormalizeEmail(identifier))).Result()
	if errors.Is(err, redis.Nil) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("lookup login: %w", err)
	}
	return p.Get(ctx, id)
}

// StartLoginOTP (dev) generates a code locally and logs it.
func (p *DevProvider) StartLoginOTP(ctx context.Context, identifier string) (LoginOTPHandle, string, error) {
	user, err := p.LookupByLogin(ctx, identifier)
	if err != nil {
		return LoginOTPHandle{}, "", err
	}
	code, err := genCode()
	if err != nil {
		return LoginOTPHandle{}, "", err
	}
	channel := "email"
	if !strings.Contains(identifier, "@") {
		channel = "mobile"
	}
	p.log.Info("DEV login code", zap.String("channel", channel), zap.String("user_id", user.ID), zap.String("code", code))
	return LoginOTPHandle{UserID: user.ID, Channel: channel, CodeHash: hashCode(code)}, code, nil
}

// VerifyLoginOTP (dev) compares the code to the stored hash in the handle.
func (p *DevProvider) VerifyLoginOTP(ctx context.Context, handle LoginOTPHandle, code string) (User, error) {
	if subtle.ConstantTimeCompare([]byte(hashCode(code)), []byte(handle.CodeHash)) != 1 {
		return User{}, ErrBadCode
	}
	return p.Get(ctx, handle.UserID)
}

// EnsureUser (dev) idempotently creates a user with a verified email.
func (p *DevProvider) EnsureUser(ctx context.Context, in RegisterInput) (User, error) {
	if u, err := p.LookupByLogin(ctx, in.Email); err == nil {
		return u, nil
	}
	user, _, err := p.Register(ctx, in)
	if errors.Is(err, ErrEmailTaken) {
		return p.LookupByLogin(ctx, in.Email)
	}
	if err != nil {
		return User{}, err
	}
	_ = p.mutate(ctx, user.ID, func(r *record) { r.EmailVerified = true })
	user.EmailVerified = true
	return user, nil
}

// ── code helpers ──────────────────────────────────────────────────

func (p *DevProvider) issueCode(ctx context.Context, channel, userID string) (string, error) {
	code, err := genCode()
	if err != nil {
		return "", err
	}
	if err := p.rdb.SetEx(ctx, codeKey(channel, userID), hashCode(code), p.codeTTL).Err(); err != nil {
		return "", fmt.Errorf("store code: %w", err)
	}
	// "Deliver" the code by logging it — the dev stand-in for email/SMS.
	p.log.Info("DEV verification code", zap.String("channel", channel), zap.String("user_id", userID), zap.String("code", code))
	return code, nil
}

func (p *DevProvider) checkCode(ctx context.Context, channel, userID, code string) error {
	want, err := p.rdb.Get(ctx, codeKey(channel, userID)).Result()
	if errors.Is(err, redis.Nil) {
		return ErrCodeExpired
	}
	if err != nil {
		return fmt.Errorf("load code: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(want), []byte(hashCode(code))) != 1 {
		return ErrBadCode
	}
	_ = p.rdb.Del(ctx, codeKey(channel, userID)).Err()
	return nil
}

func (p *DevProvider) maybe(code string) string {
	if p.returnDbg {
		return code
	}
	return ""
}

func (p *DevProvider) save(ctx context.Context, rec record) error {
	raw, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal user: %w", err)
	}
	return p.rdb.Set(ctx, userKey(rec.ID), raw, 0).Err()
}

func (p *DevProvider) load(ctx context.Context, id string) (record, error) {
	raw, err := p.rdb.Get(ctx, userKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return record{}, ErrNotFound
	}
	if err != nil {
		return record{}, fmt.Errorf("load user: %w", err)
	}
	var rec record
	if err := json.Unmarshal(raw, &rec); err != nil {
		return record{}, fmt.Errorf("unmarshal user: %w", err)
	}
	return rec, nil
}

func (p *DevProvider) mutate(ctx context.Context, id string, fn func(*record)) error {
	rec, err := p.load(ctx, id)
	if err != nil {
		return err
	}
	fn(&rec)
	return p.save(ctx, rec)
}

func genCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// SendPasswordReset (dev): resolves the user, stores a reset code, and logs the
// completion link (no real email in dev). Returns ErrNotFound when unknown.
func (p *DevProvider) SendPasswordReset(ctx context.Context, email, urlTemplate string) error {
	id, err := p.rdb.Get(ctx, emailKey(NormalizeEmail(email))).Result()
	if err != nil || id == "" {
		return ErrNotFound
	}
	code, err := genCode()
	if err != nil {
		return err
	}
	if err := p.rdb.Set(ctx, codeKey("reset", id), code, p.codeTTL).Err(); err != nil {
		return fmt.Errorf("store reset code: %w", err)
	}
	link := strings.NewReplacer("{{.UserID}}", id, "{{.Code}}", code).Replace(urlTemplate)
	p.log.Info("dev password reset (no email sent)", zap.String("email", email), zap.String("reset_link", link))
	return nil
}

// ResetPassword (dev): verifies the code and sets the new password hash.
func (p *DevProvider) ResetPassword(ctx context.Context, userID, code, newPassword string) error {
	want, err := p.rdb.Get(ctx, codeKey("reset", userID)).Result()
	if err != nil || want == "" || subtle.ConstantTimeCompare([]byte(want), []byte(code)) != 1 {
		return ErrBadCode
	}
	raw, err := p.rdb.Get(ctx, userKey(userID)).Bytes()
	if err != nil {
		return ErrNotFound
	}
	var rec record
	if err := json.Unmarshal(raw, &rec); err != nil {
		return fmt.Errorf("decode user: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	rec.PasswordHash = string(hash)
	b, _ := json.Marshal(rec)
	if err := p.rdb.Set(ctx, userKey(userID), b, 0).Err(); err != nil {
		return fmt.Errorf("save user: %w", err)
	}
	_ = p.rdb.Del(ctx, codeKey("reset", userID)).Err()
	return nil
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
