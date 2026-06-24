package identity

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// errZitadelExists is the internal sentinel for a Zitadel ALREADY_EXISTS response.
// It does NOT necessarily mean an email duplicate in our org — Zitadel usernames
// are unique across the whole instance, so it can be a username collision in a
// different org. We disambiguate with an org-scoped email lookup.
var errZitadelExists = errors.New("zitadel resource already exists")

// ZitadelProvider delegates the full identity lifecycle to the platform Zitadel
// instance — it stores passwords and SENDS the email/SMS verification codes. The
// API shapes are the v2 User API (the same instance docflow-auth uses):
//
//   Register → POST /v2/users/human            (email.sendCode → Zitadel emails the code)
//   Verify   → POST /v2/users/{id}/email/verify
//   Resend   → POST /v2/users/{id}/email/resend
//   Phone    → POST /v2/users/{id}/phone[/verify]
//   Login    → POST /v2/sessions  (+ GET /v2/sessions/{id})
//   Get      → GET  /v2/users/{id}
type ZitadelProvider struct {
	issuer     string
	servicePAT string
	orgID      string
	returnCode bool // local testing: ask Zitadel to return the code instead of emailing it
	http       *http.Client
	log        *zap.Logger
}

// ZitadelConfig carries the settings the provider needs.
type ZitadelConfig struct {
	Issuer     string // internal (server-to-server) Zitadel base URL
	ServicePAT string
	OrgID      string
	ReturnCode bool
}

// NewZitadelProvider builds the provider.
func NewZitadelProvider(cfg ZitadelConfig, log *zap.Logger) *ZitadelProvider {
	return &ZitadelProvider{
		issuer:     strings.TrimRight(cfg.Issuer, "/"),
		servicePAT: cfg.ServicePAT,
		orgID:      cfg.OrgID,
		returnCode: cfg.ReturnCode,
		http:       &http.Client{Timeout: 12 * time.Second},
		log:        log,
	}
}

// codeDelivery returns the email/phone verification delivery object: either
// returnCode{} (testing) or sendCode{} (production — Zitadel sends the message).
func (p *ZitadelProvider) codeDelivery() (key string, val any) {
	if p.returnCode {
		return "returnCode", map[string]any{}
	}
	return "sendCode", map[string]any{}
}

func (p *ZitadelProvider) Register(ctx context.Context, in RegisterInput) (User, string, error) {
	if p.servicePAT == "" {
		return User{}, "", ErrNotConfigured
	}
	email := NormalizeEmail(in.Email)

	// Duplicate check is ORG-SCOPED: only block when the email already exists in
	// THIS organization. The same email in another org is allowed.
	if uid := p.userIDByEmail(ctx, email); uid != "" {
		return User{}, "", ErrEmailTaken
	}

	// First try the email as the username (readable). If Zitadel rejects it with
	// ALREADY_EXISTS, the email isn't in our org (we just checked) — so it's an
	// instance-wide USERNAME collision. Retry with a unique username; login still
	// resolves by email, so the username value is irrelevant to the user.
	userID, code, err := p.createHuman(ctx, in, email, email)
	if errors.Is(err, errZitadelExists) {
		userID, code, err = p.createHuman(ctx, in, email, email+"-"+randomSuffix())
	}
	if errors.Is(err, errZitadelExists) {
		// Still conflicting after a unique username — treat as taken, to be safe.
		return User{}, "", ErrEmailTaken
	}
	if err != nil {
		return User{}, "", err
	}

	user := User{
		ID:          userID,
		Email:       email,
		FirstName:   in.FirstName,
		LastName:    in.LastName,
		DisplayName: in.DisplayName,
		Phone:       in.Phone,
		Company:     in.Company,
		JobTitle:    in.JobTitle,
		Role:        "member",
	}
	p.log.Info("zitadel user created; verification email sent", zap.String("user_id", user.ID))
	return user, code, nil
}

// createHuman issues a single create-human call. Returns errZitadelExists on a
// Zitadel ALREADY_EXISTS response so the caller can disambiguate.
func (p *ZitadelProvider) createHuman(ctx context.Context, in RegisterInput, email, username string) (userID, emailCode string, err error) {
	profile := map[string]any{
		"givenName":         in.FirstName,
		"familyName":        in.LastName,
		"preferredLanguage": "en",
	}
	if in.DisplayName != "" {
		profile["nickName"] = in.DisplayName
	}

	delKey, delVal := p.codeDelivery()
	body := map[string]any{
		"username": username,
		"profile":  profile,
		"email":    map[string]any{"email": email, delKey: delVal},
		"password": map[string]any{"password": in.Password, "changeRequired": false},
	}
	if p.orgID != "" {
		body["organization"] = map[string]any{"orgId": p.orgID}
	}
	if strings.HasPrefix(in.Phone, "+") {
		body["phone"] = map[string]any{"phone": in.Phone} // verified later, on demand
	}

	status, raw, err := p.do(ctx, http.MethodPost, p.issuer+"/v2/users/human", body)
	if err != nil {
		return "", "", fmt.Errorf("zitadel create user: %w", err)
	}
	if status != http.StatusOK && status != http.StatusCreated {
		s := string(raw)
		if strings.Contains(s, "ALREADY_EXISTS") || strings.Contains(s, "already exists") {
			return "", "", errZitadelExists
		}
		p.log.Warn("zitadel create user rejected", zap.Int("status", status), zap.String("body", s))
		return "", "", fmt.Errorf("zitadel create user: status %d", status)
	}

	var out struct {
		UserID    string `json:"userId"`
		EmailCode string `json:"emailCode"`
	}
	_ = json.Unmarshal(raw, &out)
	return out.UserID, out.EmailCode, nil
}

// randomSuffix returns a short random hex string for de-colliding usernames.
func randomSuffix() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000)
	}
	return hex.EncodeToString(b)
}

func (p *ZitadelProvider) VerifyEmail(ctx context.Context, userID, code string) error {
	status, raw, err := p.do(ctx, http.MethodPost,
		fmt.Sprintf("%s/v2/users/%s/email/verify", p.issuer, userID),
		map[string]any{"verificationCode": code})
	if err != nil {
		return fmt.Errorf("zitadel verify email: %w", err)
	}
	return verifyStatus(status, raw)
}

func (p *ZitadelProvider) ResendEmail(ctx context.Context, userID string) (string, error) {
	delKey, delVal := p.codeDelivery()
	status, raw, err := p.do(ctx, http.MethodPost,
		fmt.Sprintf("%s/v2/users/%s/email/resend", p.issuer, userID),
		map[string]any{delKey: delVal})
	if err != nil {
		return "", fmt.Errorf("zitadel resend email: %w", err)
	}
	if status >= 300 {
		return "", fmt.Errorf("zitadel resend email: status %d", status)
	}
	var out struct {
		EmailCode string `json:"emailCode"`
	}
	_ = json.Unmarshal(raw, &out)
	return out.EmailCode, nil
}

func (p *ZitadelProvider) StartPhoneVerification(ctx context.Context, userID string) (string, error) {
	user, err := p.Get(ctx, userID)
	if err != nil {
		return "", err
	}
	if user.Phone == "" {
		return "", fmt.Errorf("no phone on file")
	}
	delKey, delVal := p.codeDelivery()
	status, raw, err := p.do(ctx, http.MethodPost,
		fmt.Sprintf("%s/v2/users/%s/phone", p.issuer, userID),
		map[string]any{"phone": user.Phone, delKey: delVal})
	if err != nil {
		return "", fmt.Errorf("zitadel start phone verify: %w", err)
	}
	if status >= 300 {
		return "", fmt.Errorf("zitadel start phone verify: status %d", status)
	}
	var out struct {
		PhoneCode string `json:"phoneCode"`
	}
	_ = json.Unmarshal(raw, &out)
	return out.PhoneCode, nil
}

func (p *ZitadelProvider) VerifyPhone(ctx context.Context, userID, code string) error {
	status, raw, err := p.do(ctx, http.MethodPost,
		fmt.Sprintf("%s/v2/users/%s/phone/verify", p.issuer, userID),
		map[string]any{"verificationCode": code})
	if err != nil {
		return fmt.Errorf("zitadel verify phone: %w", err)
	}
	return verifyStatus(status, raw)
}

func (p *ZitadelProvider) Authenticate(ctx context.Context, email, password string) (User, error) {
	if p.servicePAT == "" {
		return User{}, ErrNotConfigured
	}
	userCheck := map[string]any{"loginName": NormalizeEmail(email)}
	if uid := p.userIDByEmail(ctx, NormalizeEmail(email)); uid != "" {
		userCheck = map[string]any{"userId": uid}
	}

	status, raw, err := p.do(ctx, http.MethodPost, p.issuer+"/v2/sessions", map[string]any{
		"checks": map[string]any{
			"user":     userCheck,
			"password": map[string]any{"password": password},
		},
		"lifetime": "3600s",
	})
	if err != nil {
		return User{}, fmt.Errorf("zitadel session: %w", err)
	}
	if status != http.StatusOK && status != http.StatusCreated {
		if isCredentialError(status, string(raw)) {
			return User{}, ErrInvalidCredentials
		}
		return User{}, fmt.Errorf("zitadel session: status %d: %s", status, string(raw))
	}
	var created struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &created); err != nil || created.SessionID == "" {
		return User{}, fmt.Errorf("zitadel session: parse response")
	}

	getStatus, getRaw, err := p.do(ctx, http.MethodGet, fmt.Sprintf("%s/v2/sessions/%s", p.issuer, created.SessionID), nil)
	if err != nil {
		return User{}, fmt.Errorf("zitadel get session: %w", err)
	}
	if getStatus != http.StatusOK {
		return User{}, fmt.Errorf("zitadel get session: status %d", getStatus)
	}
	var sd struct {
		Session struct {
			Factors struct {
				User struct {
					ID        string `json:"id"`
					HumanUser struct {
						Profile struct {
							GivenName  string `json:"givenName"`
							FamilyName string `json:"familyName"`
							NickName   string `json:"nickName"`
						} `json:"profile"`
						Email struct {
							Email      string `json:"email"`
							IsVerified bool   `json:"isVerified"`
						} `json:"email"`
						Phone struct {
							Phone      string `json:"phone"`
							IsVerified bool   `json:"isVerified"`
						} `json:"phone"`
					} `json:"humanUser"`
				} `json:"user"`
			} `json:"factors"`
		} `json:"session"`
	}
	if err := json.Unmarshal(getRaw, &sd); err != nil {
		return User{}, fmt.Errorf("zitadel get session: parse")
	}
	u := sd.Session.Factors.User
	return User{
		ID:            u.ID,
		Email:         firstNonEmpty(u.HumanUser.Email.Email, NormalizeEmail(email)),
		FirstName:     u.HumanUser.Profile.GivenName,
		LastName:      u.HumanUser.Profile.FamilyName,
		DisplayName:   u.HumanUser.Profile.NickName,
		Phone:         u.HumanUser.Phone.Phone,
		Role:          "member",
		EmailVerified: u.HumanUser.Email.IsVerified,
		PhoneVerified: u.HumanUser.Phone.IsVerified,
	}, nil
}

func (p *ZitadelProvider) Get(ctx context.Context, id string) (User, error) {
	status, raw, err := p.do(ctx, http.MethodGet, fmt.Sprintf("%s/v2/users/%s", p.issuer, id), nil)
	if err != nil {
		return User{}, fmt.Errorf("zitadel get user: %w", err)
	}
	if status == http.StatusNotFound {
		return User{}, ErrNotFound
	}
	if status != http.StatusOK {
		return User{}, fmt.Errorf("zitadel get user: status %d", status)
	}
	var out struct {
		User struct {
			UserID string `json:"userId"`
			Human  struct {
				Profile struct {
					GivenName   string `json:"givenName"`
					FamilyName  string `json:"familyName"`
					NickName    string `json:"nickName"`
					DisplayName string `json:"displayName"`
				} `json:"profile"`
				Email struct {
					Email      string `json:"email"`
					IsVerified bool   `json:"isVerified"`
				} `json:"email"`
				Phone struct {
					Phone      string `json:"phone"`
					IsVerified bool   `json:"isVerified"`
				} `json:"phone"`
			} `json:"human"`
		} `json:"user"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return User{}, fmt.Errorf("zitadel get user: parse")
	}
	h := out.User.Human
	return User{
		ID:            out.User.UserID,
		Email:         h.Email.Email,
		FirstName:     h.Profile.GivenName,
		LastName:      h.Profile.FamilyName,
		DisplayName:   firstNonEmpty(h.Profile.NickName, h.Profile.DisplayName),
		Phone:         h.Phone.Phone,
		Role:          "member",
		EmailVerified: h.Email.IsVerified,
		PhoneVerified: h.Phone.IsVerified,
	}, nil
}

func (p *ZitadelProvider) userIDByEmail(ctx context.Context, email string) string {
	status, raw, err := p.do(ctx, http.MethodPost, p.issuer+"/management/v1/users/_search", map[string]any{
		"queries": []map[string]any{
			{"emailQuery": map[string]any{"emailAddress": email, "method": "TEXT_QUERY_METHOD_EQUALS"}},
		},
	})
	if err != nil || status != http.StatusOK {
		return ""
	}
	var result struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || len(result.Result) == 0 {
		return ""
	}
	return result.Result[0].ID
}

// do performs an authenticated Zitadel request, retrying on 429/5xx (3 attempts).
func (p *ZitadelProvider) do(ctx context.Context, method, url string, body any) (int, []byte, error) {
	var buf []byte
	if body != nil {
		buf, _ = json.Marshal(body)
	}
	delays := []time.Duration{0, 300 * time.Millisecond, 900 * time.Millisecond}
	var lastErr error
	for i, delay := range delays {
		if delay > 0 {
			time.Sleep(delay)
		}
		var reader io.Reader
		if buf != nil {
			reader = bytes.NewReader(buf)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, reader)
		if err != nil {
			return 0, nil, fmt.Errorf("build request: %w", err)
		}
		if buf != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bearer "+p.servicePAT)
		if p.orgID != "" {
			req.Header.Set("x-zitadel-orgid", p.orgID)
		}
		resp, err := p.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("attempt %d: status %d", i+1, resp.StatusCode)
			continue
		}
		return resp.StatusCode, raw, nil
	}
	return 0, nil, fmt.Errorf("all attempts failed: %w", lastErr)
}

func verifyStatus(status int, raw []byte) error {
	if status >= 200 && status < 300 {
		return nil
	}
	s := string(raw)
	if strings.Contains(s, "expired") || strings.Contains(s, "Expired") {
		return ErrCodeExpired
	}
	return ErrBadCode
}

func isCredentialError(status int, body string) bool {
	if status == http.StatusUnauthorized {
		return true
	}
	for _, s := range []string{"password", "Password", "Errors.User", "could not be found", "User not found", "invalid_grant", "user not found"} {
		if strings.Contains(body, s) {
			return true
		}
	}
	return false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
