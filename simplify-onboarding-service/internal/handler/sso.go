package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/simplify/onboarding/internal/identity"
	"go.uber.org/zap"
)

// Cookies that carry SSO flow state across the IdP round-trip.
const (
	ssoReturnCookie = "sso_return" // product URL to land on afterwards
	ssoPopupCookie  = "sso_popup"  // "1" when the flow runs in a popup window
)

// SSOStart begins federated SSO (Google / Microsoft) via Zitadel's IDP Intent API.
//
// It asks Zitadel for an authUrl that points the browser straight at the external
// IdP, remembering — in short cookies — which product to return to and whether the
// flow is running in a popup. IdP IDs come from IDP_<NAME>_ID env vars.
func (h *Handler) SSOStart(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))
	idpID := h.cfg.IDPs[provider]
	if idpID == "" {
		h.ssoFail(w, r, "sso_unconfigured")
		return
	}
	if h.cfg.ZitadelServicePAT == "" {
		h.ssoFail(w, r, "sso_unavailable")
		return
	}

	// Remember where to land afterwards (the product that sent the user here).
	if rd := h.safeReturnURL(r.URL.Query().Get("redirect")); rd != "" {
		h.setFlowCookie(w, ssoReturnCookie, rd)
	}
	// Remember whether we're in a popup so the callback can close it cleanly.
	if r.URL.Query().Get("display") == "popup" {
		h.setFlowCookie(w, ssoPopupCookie, "1")
	}

	payload := map[string]any{
		"idpId": idpID,
		"urls": map[string]string{
			"successUrl": h.ssoCallbackURL(provider),
			"failureUrl": h.cfg.FrontendURL + "/auth?mode=signin&error=sso_failed",
		},
	}
	raw, status, err := h.zitadelPost(r.Context(), "/v2/idp_intents", payload, false)
	if err != nil {
		h.log.Error("sso start: zitadel unreachable", zap.Error(err))
		h.ssoFail(w, r, "sso_failed")
		return
	}
	if status != http.StatusOK && status != http.StatusCreated {
		h.log.Warn("sso start: intent rejected", zap.Int("status", status), zap.String("body", string(raw)))
		h.ssoFail(w, r, "sso_failed")
		return
	}

	var out struct {
		AuthURL string `json:"authUrl"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.AuthURL == "" {
		h.log.Error("sso start: parse intent response", zap.String("body", string(raw)))
		h.ssoFail(w, r, "sso_failed")
		return
	}
	http.Redirect(w, r, out.AuthURL, http.StatusFound)
}

// SSOCallback completes the IDP Intent: it retrieves the federated identity,
// provisions/links the Zitadel account on first login, maps it to the shared
// directory, and establishes the central SSO session — then returns the user to the
// product they came from (or closes the popup).
func (h *Handler) SSOCallback(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))
	if h.cfg.IDPs[provider] == "" {
		h.ssoFail(w, r, "unknown_provider")
		return
	}

	intentID := firstNonEmpty(r.URL.Query().Get("id"), r.URL.Query().Get("idpIntentId"))
	intentToken := firstNonEmpty(r.URL.Query().Get("token"), r.URL.Query().Get("idpIntentToken"))
	if intentID == "" || intentToken == "" {
		h.ssoFail(w, r, "sso_failed")
		return
	}

	raw, status, err := h.zitadelPost(r.Context(),
		fmt.Sprintf("/v2/idp_intents/%s", intentID),
		map[string]string{"idpIntentToken": intentToken}, false)
	if err != nil {
		h.log.Error("sso callback: zitadel unreachable", zap.Error(err))
		h.ssoFail(w, r, "sso_failed")
		return
	}
	if status != http.StatusOK {
		h.log.Warn("sso callback: retrieve intent failed", zap.Int("status", status), zap.String("body", string(raw)))
		h.ssoFail(w, r, "sso_failed")
		return
	}

	// The intent gives us EITHER a linked Zitadel user (user.id) OR — on first login —
	// just the external identity (idpInformation) to provision from.
	var data struct {
		User struct {
			ID    string `json:"id"`
			Human struct {
				Email   struct{ Email string `json:"email"` } `json:"email"`
				Profile struct {
					GivenName   string `json:"givenName"`
					FamilyName  string `json:"familyName"`
					DisplayName string `json:"displayName"`
				} `json:"profile"`
			} `json:"human"`
		} `json:"user"`
		IDPInformation struct {
			IdpID          string          `json:"idpId"`
			UserID         string          `json:"userId"`
			UserName       string          `json:"userName"` // the email / UPN
			RawInformation json.RawMessage `json:"rawInformation"`
		} `json:"idpInformation"`
		// Zitadel normalises the external identity into these create templates
		// (provider-agnostic — works for Google, Microsoft, etc.). Present only when
		// the user isn't linked yet (first login).
		AddHumanUser humanTemplate `json:"addHumanUser"`
		CreateUser   struct {
			Username string        `json:"username"`
			Human    humanTemplate `json:"human"`
		} `json:"createUser"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		h.log.Error("sso callback: parse intent data", zap.Error(err))
		h.ssoFail(w, r, "sso_parse_failed")
		return
	}

	sub := data.User.ID
	email := identity.NormalizeEmail(data.User.Human.Email.Email)
	given := data.User.Human.Profile.GivenName
	family := data.User.Human.Profile.FamilyName
	display := data.User.Human.Profile.DisplayName

	if sub == "" {
		// First-time federated login — provision/link from Zitadel's normalised
		// template (addHumanUser, falling back to createUser.human).
		tmpl := data.AddHumanUser
		if tmpl.Email.Email == "" {
			tmpl = data.CreateUser.Human
			if tmpl.Username == "" {
				tmpl.Username = data.CreateUser.Username
			}
		}
		email = identity.NormalizeEmail(tmpl.Email.Email)
		given = tmpl.Profile.GivenName
		family = tmpl.Profile.FamilyName
		display = tmpl.Profile.DisplayName

		// Some IdP configs return only the bare external identity (no linked user, no
		// create template). Fall back to idpInformation: userName is the email/UPN, and
		// rawInformation holds the profile (provider-specific keys — handled below).
		if email == "" {
			email = identity.NormalizeEmail(data.IDPInformation.UserName)
		}
		if given == "" || family == "" || display == "" {
			rg, rf, rd, re := parseRawProfile(data.IDPInformation.RawInformation)
			given = orFallback(given, rg)
			family = orFallback(family, rf)
			display = orFallback(display, rd)
			if email == "" {
				email = identity.NormalizeEmail(re)
			}
		}

		if email == "" {
			h.log.Error("sso callback: no email in intent",
				zap.String("idp_userName", data.IDPInformation.UserName),
				zap.String("raw", truncate(string(data.IDPInformation.RawInformation), 400)))
			h.ssoFail(w, r, "sso_parse_failed")
			return
		}

		fed := federatedUser{
			email:     email,
			given:     given,
			family:    family,
			display:   display,
			idpID:     data.IDPInformation.IdpID,
			extUserID: data.IDPInformation.UserID,
			userName:  firstNonEmpty(tmpl.Username, data.IDPInformation.UserName, email),
		}
		if len(tmpl.IDPLinks) > 0 {
			if tmpl.IDPLinks[0].IdpID != "" {
				fed.idpID = tmpl.IDPLinks[0].IdpID
			}
			if tmpl.IDPLinks[0].UserID != "" {
				fed.extUserID = tmpl.IDPLinks[0].UserID
			}
		}

		sub, err = h.ensureFederatedUser(r.Context(), fed)
		if err != nil {
			h.log.Error("sso callback: provision federated user", zap.Error(err))
			h.ssoFail(w, r, "sso_failed")
			return
		}
	}

	user := identity.User{
		ID:            sub, // Zitadel user id
		Email:         email,
		FirstName:     given,
		LastName:      family,
		DisplayName:   display,
		Role:          "member",
		EmailVerified: true, // the IdP has verified the email
	}
	if err := h.establishSession(w, r, user, false, provider); err != nil {
		h.log.Error("sso callback: create session", zap.Error(err))
		h.ssoFail(w, r, "session")
		return
	}

	h.log.Info("sso login", zap.String("provider", provider), zap.String("zitadel_sub", sub))

	dest := h.consumeReturnURL(w, r)
	if h.consumePopup(w, r) {
		h.renderSSOResult(w, true, dest, "")
		return
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

// humanTemplate is Zitadel's normalised representation of an external identity,
// used to create/link the local Zitadel account on first federated login.
type humanTemplate struct {
	Username string `json:"username"`
	Profile  struct {
		GivenName   string `json:"givenName"`
		FamilyName  string `json:"familyName"`
		DisplayName string `json:"displayName"`
	} `json:"profile"`
	Email struct {
		Email string `json:"email"`
	} `json:"email"`
	IDPLinks []struct {
		IdpID  string `json:"idpId"`
		UserID string `json:"userId"`
	} `json:"idpLinks"`
}

// ── federated provisioning ────────────────────────────────────────

type federatedUser struct {
	email, given, family, display string
	idpID, extUserID, userName    string
}

// ensureFederatedUser returns the Zitadel user id for a federated identity,
// creating the linked account on first login (or reusing an existing one with the
// same email — e.g. someone who first registered with a password).
func (h *Handler) ensureFederatedUser(ctx context.Context, f federatedUser) (string, error) {
	given := orFallback(f.given, localPart(f.email))
	family := orFallback(f.family, given)
	body := map[string]any{
		"username": f.userName,
		"profile":  map[string]any{"givenName": given, "familyName": family, "displayName": orFallback(f.display, given)},
		"email":    map[string]any{"email": f.email, "isVerified": true},
		"idpLinks": []map[string]any{{"idpId": f.idpID, "userId": f.extUserID, "userName": f.userName}},
	}
	raw, status, err := h.zitadelPost(ctx, "/v2/users/human", body, true)
	if err != nil {
		return "", err
	}
	if status == http.StatusOK || status == http.StatusCreated {
		var out struct {
			UserID string `json:"userId"`
		}
		if json.Unmarshal(raw, &out); out.UserID != "" {
			return out.UserID, nil
		}
	}
	// Username/email already exists → this email already has an account (e.g. it was
	// created with a password). Reuse it AND link the IdP so the two sign-in methods
	// resolve to ONE account — the practical "sign in with Google OR password" model.
	if id := h.userIDByEmail(ctx, f.email); id != "" {
		h.linkIDP(ctx, id, f)
		return id, nil
	}
	return "", fmt.Errorf("create federated user (status %d): %s", status, truncate(string(raw), 300))
}

// linkIDP attaches a federated identity to an existing Zitadel user (best-effort —
// sign-in still works via email match even if the link already exists / fails).
func (h *Handler) linkIDP(ctx context.Context, userID string, f federatedUser) {
	body := map[string]any{
		"idpLink": map[string]any{"idpId": f.idpID, "userId": f.extUserID, "userName": f.userName},
	}
	raw, status, err := h.zitadelPost(ctx, "/v2/users/"+userID+"/links", body, true)
	if err != nil {
		h.log.Warn("sso: link idp failed", zap.Error(err))
		return
	}
	if status != http.StatusOK && status != http.StatusCreated {
		// ALREADY_EXISTS (already linked) is fine; anything else is just logged.
		if !strings.Contains(string(raw), "already") && !strings.Contains(string(raw), "ALREADY") {
			h.log.Warn("sso: link idp non-ok", zap.Int("status", status), zap.String("body", truncate(string(raw), 200)))
		}
	}
}

// userIDByEmail finds a Zitadel user id by email within the configured org.
func (h *Handler) userIDByEmail(ctx context.Context, email string) string {
	body := map[string]any{
		"queries": []map[string]any{
			{"emailQuery": map[string]any{"emailAddress": email, "method": "TEXT_QUERY_METHOD_EQUALS_IGNORE_CASE"}},
		},
	}
	raw, status, err := h.zitadelPost(ctx, "/management/v1/users/_search", body, true)
	if err != nil || status != http.StatusOK {
		return ""
	}
	var out struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &out); len(out.Result) > 0 {
		return out.Result[0].ID
	}
	return ""
}

// zitadelPost performs an authenticated POST to Zitadel, optionally scoped to the org.
func (h *Handler) zitadelPost(ctx context.Context, path string, payload any, withOrg bool) ([]byte, int, error) {
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.cfg.ZitadelInternalURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.cfg.ZitadelServicePAT)
	if withOrg && h.cfg.ZitadelOrgID != "" {
		req.Header.Set("x-zitadel-orgid", h.cfg.ZitadelOrgID)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return raw, resp.StatusCode, nil
}

// ── popup / redirect helpers ──────────────────────────────────────

// renderSSOResult returns a tiny page that hands control back to the opener window
// (the portal) and closes the popup; it falls back to navigating if close is blocked.
func (h *Handler) renderSSOResult(w http.ResponseWriter, ok bool, dest, errCode string) {
	fallback := dest
	if !ok {
		fallback = strings.TrimRight(h.cfg.FrontendURL, "/") + "/auth?mode=signin&error=" + errCode
	}
	msgJS, _ := json.Marshal(map[string]any{
		"source": "simplify-sso", "ok": ok, "redirect": dest, "error": errCode,
	})
	fbJS, _ := json.Marshal(fallback)
	originJS, _ := json.Marshal(strings.TrimRight(h.cfg.FrontendURL, "/"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>Signing you in…</title></head>
<body style="font-family:system-ui,-apple-system,sans-serif;display:grid;place-items:center;height:100vh;margin:0;color:#475569">
<p>Signing you in…</p>
<script>(function(){
  var msg=%s, fb=%s, origin=%s;
  try{ if(window.opener){ window.opener.postMessage(msg, origin); } }catch(e){}
  window.close();
  setTimeout(function(){ try{ window.location.replace(fb); }catch(e){} }, 400);
})();</script>
</body></html>`, msgJS, fbJS, originJS)
}

// isPopup reports whether the current flow is running in a popup (query on start,
// cookie on the callback round-trip).
func (h *Handler) isPopup(r *http.Request) bool {
	if r.URL.Query().Get("display") == "popup" {
		return true
	}
	if c, err := r.Cookie(ssoPopupCookie); err == nil && c.Value == "1" {
		return true
	}
	return false
}

func (h *Handler) setFlowCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/", MaxAge: 600,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: h.cfg.CookieSecure,
	})
}

func (h *Handler) clearFlowCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: h.cfg.CookieSecure,
	})
}

// ssoCallbackURL is the browser-reachable callback for a provider (served by the
// portal, which proxies /auth/* to this service).
func (h *Handler) ssoCallbackURL(provider string) string {
	return strings.TrimRight(h.cfg.FrontendURL, "/") + "/auth/sso/callback/" + provider
}

// ssoFail returns the user to the portal sign-in with an error code. In popup mode it
// closes the popup and the OPENER shows the error (instead of rendering the whole
// portal inside the little window).
func (h *Handler) ssoFail(w http.ResponseWriter, r *http.Request, code string) {
	popup := h.isPopup(r)
	h.clearFlowCookie(w, ssoPopupCookie)
	h.clearFlowCookie(w, ssoReturnCookie)
	if popup {
		h.renderSSOResult(w, false, "", code)
		return
	}
	http.Redirect(w, r, h.cfg.FrontendURL+"/auth?mode=signin&error="+code, http.StatusFound)
}

// consumeReturnURL reads (and clears) the post-SSO return cookie, falling back to the
// portal home. The value was validated when it was set.
func (h *Handler) consumeReturnURL(w http.ResponseWriter, r *http.Request) string {
	dest := strings.TrimRight(h.cfg.FrontendURL, "/") + "/"
	if c, err := r.Cookie(ssoReturnCookie); err == nil && c.Value != "" {
		if rd := h.safeReturnURL(c.Value); rd != "" {
			dest = rd
		}
		h.clearFlowCookie(w, ssoReturnCookie)
	}
	return dest
}

// consumePopup reports (and clears) whether this flow was started in a popup.
func (h *Handler) consumePopup(w http.ResponseWriter, r *http.Request) bool {
	c, err := r.Cookie(ssoPopupCookie)
	if err != nil || c.Value != "1" {
		return false
	}
	h.clearFlowCookie(w, ssoPopupCookie)
	return true
}

// safeReturnURL returns raw only when its origin is in the allowed list — so the
// post-login redirect can never be an arbitrary (open-redirect) destination.
func (h *Handler) safeReturnURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return ""
	}
	origin := u.Scheme + "://" + u.Host
	for _, o := range h.cfg.CORSAllowedOrigins {
		if strings.EqualFold(strings.TrimRight(o, "/"), origin) {
			return raw
		}
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func orFallback(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func localPart(email string) string {
	if i := strings.IndexByte(email, '@'); i > 0 {
		return email[:i]
	}
	return email
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// parseRawProfile extracts name/email from an IdP's raw claims, tolerating both
// Google's shape (nested under "User": given_name/family_name/name/email) and
// Microsoft's (flat: givenName/surname/displayName/mail/userPrincipalName).
func parseRawProfile(raw json.RawMessage) (given, family, display, email string) {
	if len(raw) == 0 {
		return
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return
	}
	if u, ok := m["User"].(map[string]any); ok {
		m = u // Google nests the profile under "User"
	}
	pick := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
				return v
			}
		}
		return ""
	}
	given = pick("given_name", "givenName")
	family = pick("family_name", "familyName", "surname")
	display = pick("name", "displayName")
	email = pick("email", "mail", "userPrincipalName")
	return
}
