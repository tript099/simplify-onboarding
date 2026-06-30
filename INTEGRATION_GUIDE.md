# Simplify Onboarding — Product Integration Guide

> How to plug any Simplify product into the central **Simplify Onboarding** platform so
> users get **one account, one sign-in, every product** — the Google-style SSO model.
>
> Audience: backend + frontend engineers integrating a new product (e.g. SimplifyLegal,
> SimplifyInsights) the same way **DocFlow** is integrated.

---

## 1. What this platform is (and is not)

**Simplify Onboarding is a standalone identity & account layer.** It answers exactly one
question for every product: **"who is this browser?"** — and returns a single, stable,
universal identity.

It is **authentication (authN)**. It is **not** authorization.

| Onboarding owns | Each product owns |
|---|---|
| Sign-up / sign-in / SSO (Google, Microsoft) | Its own users table / internal user id |
| Email & phone OTP verification | Roles, permissions, tenants (authZ) |
| The central session (the `session_id` cookie) | Product data |
| The **universal identity** = the Zitadel `sub` | Mapping `sub → its own internal user id` |

### The one rule

> **The universal identity is the Zitadel `sub`.** Every product maps that `sub` to its
> own internal user id on its own side. The onboarding platform never stores, and never
> needs to know, any product's internal ids, tenants, or roles.

This is why the platform is **fully standalone**: its only external dependency is Zitadel
(the shared IdP). It has its **own** Redis (sessions) and its **own** Postgres (profiles,
demo requests, analytics, audit).

---

## 2. Architecture

```
                            ┌──────────────────────────────┐
                            │   Zitadel  (shared IdP)       │
                            │   auth.simplifyaipro.com      │
                            │   - passwords, OTP, MFA       │
                            │   - Google / Microsoft IdPs   │
                            └───────────────▲──────────────┘
                                            │ OIDC + Mgmt API (service PAT)
                                            │  (the ONLY external dependency)
                                            │
   ┌────────┐   session_id cookie  ┌────────┴───────────────────────┐
   │Browser │◄────────────────────►│   SIMPLIFY ONBOARDING SERVICE   │  ← STANDALONE
   └────────┘                      │   Go · :8090                    │
        │                          │   /auth/*   /onb/*   /validate  │
        │ 1. redirect to portal    │                                 │
        │    when not signed in    │   ┌──────────┐   ┌───────────┐  │
        │                          │   │ Redis    │   │ Postgres  │  │
        │                          │   │ sessions │   │ users     │  │
        │                          │   │ OTP      │   │ demo_reqs │  │
        │                          │   │ visitors │   │ events    │  │
        │                          │   └──────────┘   │ login_audit│ │
        │                          └────────▲─────────└───────────┘──┘
        │                                   │
        │ 2. after login, cookie is set     │  GET /auth/validate   (cookie forwarded)
        │    (shared across products)       │  →  { valid, sub, email, names, ... }
        ▼                                   │
   ┌─────────────────┬───────────────┬──────┴────────┬─────────────────┐
   │   DocFlow       │  SimplifyLegal │  Insights     │   ...more       │
   │  (reference)    │                │               │                 │
   │  own DB + RBAC  │  own DB + RBAC │ own DB + RBAC │  own DB + RBAC  │
   └─────────────────┴───────────────┴───────────────┴─────────────────┘
        each product: validates the cookie → gets `sub` → maps to ITS OWN user id
```

**Key properties**

- **One cookie, many products.** The `session_id` cookie is set by the onboarding
  service and shared across products (same parent domain in prod; host-only `localhost`
  across ports in dev). Every product reads the *same* cookie.
- **Validation is over HTTP**, not a shared datastore. Products never read the onboarding
  Redis/DB — they call `GET /auth/validate`.
- **Persist-until-logout sessions.** The central session does not expire on its own; only
  explicit logout removes it (`SESSION_PERSIST=true`).

---

## 3. The integration contract

A product integrates by implementing **three things**:

1. **Redirect to the portal** when the user is not signed in.
2. **Validate the session** by calling `GET /auth/validate` with the cookie, and
   **map the returned `sub` to its own internal user**.
3. **Logout** via the central logout endpoint.

### 3.1 Endpoints you consume

| Method & path | Purpose | Auth |
|---|---|---|
| `GET /auth/validate` | **The integration endpoint.** Validate the session cookie; returns the universal identity. | cookie `session_id` |
| `GET /auth/me` | Same identity in a "current user" shape (handy for a frontend proxy). | cookie `session_id` |
| `GET /auth/logout` | Destroy the central session + clear the cookie (browser redirect). | cookie |
| `POST /auth/logout` | Same, JSON response (for API clients). | cookie |

You generally **do not** call the sign-up / sign-in endpoints — the **portal UI** owns
those. Your product only ever *validates* and *logs out*.

### 3.2 `GET /auth/validate` — request / response

**Request** — just forward the browser's cookie:

```http
GET /auth/validate HTTP/1.1
Host: onboarding:8090
Cookie: session_id=<value>
```

**Response (200)** — the universal identity. Note: **no product-specific fields**
(no internal id, no tenant, no role — those are yours to derive):

```json
{
  "valid": true,
  "sub": "378939800046010371",      // ← THE universal identity (Zitadel subject)
  "email": "ravi.k@simplifyai.id",
  "email_verified": true,
  "first_name": "Ravi",
  "last_name": "Kumar",
  "display_name": "Ravi Kumar",
  "phone": "+91…"
}
```

**Response (401)** — not signed in / session expired:

```json
{ "valid": false }
```

### 3.3 What your product does with the response

```
sub  ─────────────►  upsert into YOUR users table (key by sub)  ─────►  your_internal_user_id
email, names ─────►  store/refresh the profile on that row
                                                                          │
your_internal_user_id  ─────►  derive YOUR tenant + roles + permissions ─┘
```

The platform hands you a verified identity; **you** decide what that user can do.

---

## 4. Request flows

### 4.1 First visit — sign in once, land back on the product

```
Browser            Product (frontend)        Onboarding Portal         Onboarding API
   │  open product       │                          │                        │
   │────────────────────►│                          │                        │
   │                     │ not authenticated        │                        │
   │  302 redirect:  ${ONBOARDING_URL}/auth?mode=signin&redirect=<product-url>│
   │◄────────────────────┤                          │                        │
   │  GET portal /auth?redirect=…                    │                        │
   │────────────────────────────────────────────────►│                        │
   │  user signs in (password / OTP / Google / MS)   │  POST /auth/login etc. │
   │─────────────────────────────────────────────────────────────────────────►│
   │                                                 │   creates session in   │
   │                                                 │   onboarding's Redis,  │
   │              Set-Cookie: session_id=…  (shared) │   keyed by sub         │
   │◄─────────────────────────────────────────────────────────────────────────┤
   │  portal redirects browser back to <product-url> │                        │
   │◄────────────────────────────────────────────────┤                        │
   │  open product (now WITH cookie)                 │                        │
   │────────────────────►│                          │                        │
   │                     │ validate cookie ─────────────────────────────────► │ GET /auth/validate
   │                     │                          │   { valid, sub, … } ◄── │
   │                     │ map sub → internal user, derive tenant/roles        │
   │   product loads, signed in  ◄───────────────────┤                        │
```

### 4.2 Second product — no second login (the SSO payoff)

```
Browser already has session_id cookie (set on first login above)
   │  open ANOTHER product (e.g. SimplifyLegal)
   │──────────────────────────────►│ validate cookie ─────────► GET /auth/validate
   │                               │     { valid, sub, … } ◄──
   │                               │ map sub → Legal's own user id
   │  signed in instantly ◄────────┤   (no login screen at all)
```

### 4.3 Federated SSO (Google / Microsoft)

The portal owns this entirely; products see only the resulting session. For reference:

```
Portal  ──POST /v2/idp_intents──►  Zitadel  ──►  Google/Microsoft  (popup)
   ▲                                                   │
   │  Set-Cookie session_id (universal sub)            │ user authenticates
   └───  GET /auth/sso/callback/{provider}  ◄──────────┘
         (provisions/links the Zitadel user, opens central session)
```

### 4.4 Logout

```
Product "Sign out"
   │  GET ${ONBOARDING_URL}/auth/logout   (or POST for JSON)
   │────────────────────────────────────────────────► destroys central session,
   │                                                    clears session_id cookie
   │  redirect to portal sign-in  ◄─────────────────── (browser lands on portal)
```

Because the cookie is shared, clearing it logs the user out of **every** product.

---

## 5. Integrating a new product — step by step

### Step 1 — Share the cookie

The browser must send the **same** `session_id` cookie to both the portal and your product.

- **Production:** set `COOKIE_DOMAIN=.simplifyaipro.com` on the onboarding service, and
  host your product on a subdomain (e.g. `legal.simplifyaipro.com`). The cookie is then
  sent to all subdomains.
- **Local dev:** the cookie is host-only for `localhost` and is shared across ports
  (`localhost:3100` portal, `localhost:3000` product). Nothing to configure.

### Step 2 — Redirect unauthenticated users to the portal (frontend)

When your app loads and has no valid session, send the browser to the portal, carrying a
`redirect` back to where the user was:

```ts
const ONBOARDING_URL = import.meta.env.VITE_ONBOARDING_URL; // e.g. https://app.simplifyaipro.com
function redirectToLogin() {
  const ret = encodeURIComponent(window.location.href);
  window.location.replace(`${ONBOARDING_URL}/auth?mode=signin&redirect=${ret}`);
}
```

> Your product should **never render its own login/signup page.** All sign-in happens on
> the portal. (In DocFlow we removed the local `/auth` page and bounce it to the portal.)

### Step 3 — Validate the session + map the sub (backend)

On every protected request, validate the cookie and resolve the `sub` to your own user.
Pseudocode:

```go
// 1. Forward the cookie to the onboarding validate endpoint.
resp := GET(ONBOARDING_VALIDATE_URL, cookie=req.Cookie("session_id"))
if resp.status != 200 || !resp.valid {
    return 401 // → frontend redirects to portal (Step 2)
}

// 2. Map the universal sub to YOUR internal user (upsert by sub), syncing the profile.
internalUser := db.UpsertUserBySub(resp.sub, resp.email, resp.first_name, resp.last_name, …)

// 3. Derive YOUR authorization context from YOUR data.
tenant, roles := db.ResolveContext(internalUser.id)

// 4. Inject into the request context / headers for downstream handlers.
ctx = withUser(ctx, internalUser.id, tenant, roles)
```

Keep a small **`sub → internal_user_id`** upsert in your own DB. That table is your
product's source of truth for identity; the platform's `sub` is the stable foreign key.

### Step 4 — Logout

Point your "Sign out" action at the central logout, then land on the portal:

```ts
await fetch(`${ONBOARDING_URL}/auth/logout`, { credentials: 'include' });
window.location.href = `${ONBOARDING_URL}/auth?mode=signin`;
```

### Step 5 — (Optional) "Open in product" links from the portal

The portal's product cards can deep-link into your app (`launchUrl`). Because the cookie
is already set, the user lands signed in. No extra work on your side.

---

## 6. Reference implementation — how DocFlow does it

DocFlow is the canonical integration. Use it as a copy-paste template.

**Frontend (`docflow-frontend`)**
- `ProtectedRoute.tsx` → unauthenticated users are redirected to the portal
  (`redirectToOnboardingLogin()` in `services/zitadelAuth.ts`).
- The local `/auth` route renders a redirect-to-portal component, not a login form.
- Sign-out and session-timeout both route to the portal (`onboardingLoginUrl()`).
- `VITE_ONBOARDING_URL` env points at the portal.

**Backend — validation lives in `docflow-auth` (reused by the BFF):**
- `internal/handler/handler.go`
  - `fetchCentralIdentity()` — `GET ONBOARDING_VALIDATE_URL` with the cookie → `{sub, email, names, phone}`.
  - `resolveCentral()` — maps the sub to DocFlow's user via
    `userstore.ResolveOrCreateWithName(sub, email, firstName, lastName, …, {NickName, Phone})`
    (this is what keeps `app_users` populated with the full profile, not just the id).
  - `/validate` (`resolveCentralSession`) and `/me` (`meFromCentral`) both fall back to
    onboarding when the cookie isn't a docflow-native session.
- `internal/config/config.go` → `ONBOARDING_VALIDATE_URL` (default
  `http://onboarding:8090/auth/validate`).

**BFF (`docflow-bff`)**
- `internal/middleware/auth.go` → `callValidate()` **forwards the `session_id` cookie** to
  docflow-auth so cookie-only requests reach the onboarding fallback.

**Compose / networking**
- The onboarding service joins DocFlow's network with the alias `onboarding`, so
  `http://onboarding:8090/auth/validate` is reachable.
- `ONBOARDING_VALIDATE_URL` is set on `docflow-auth` in the root `docker-compose.yml`.

> A new product can either replicate this in its own auth service (like docflow-auth) or
> do the validate-and-map directly in its gateway/BFF. Both are fine — the contract is
> identical: **cookie → `/auth/validate` → `sub` → your user.**

---

## 7. Configuration reference

### Onboarding service (the platform)

| Env | Meaning |
|---|---|
| `REDIS_URL`, `REDIS_DB` | Its **own** session store. |
| `ONBOARDING_DATABASE_URL` | Its **own** Postgres (users, demo requests, events, audit). |
| `IDENTITY_PROVIDER=zitadel` | Use Zitadel (the only external dependency). |
| `ZITADEL_ISSUER` / `ZITADEL_INTERNAL_URL` / `ZITADEL_CLIENT_ID` / `ZITADEL_SERVICE_PAT` / `ZITADEL_ORG_ID` | Zitadel connection. |
| `IDP_GOOGLE_ID`, `IDP_MICROSOFT_ID` | Federated SSO IdP ids. |
| `SESSION_PERSIST=true` | Session lives until logout. |
| `COOKIE_DOMAIN` | `.simplifyaipro.com` in prod (share across subdomains); empty in dev. |
| `COOKIE_SECURE` | `true` in prod (HTTPS). |
| `FRONTEND_URL` | The portal SPA base (used for SSO callbacks + redirects). |
| `CORS_ALLOWED_ORIGINS` | Portal + product origins allowed to call it / be redirect targets. |

### A consuming product

| Env | Meaning |
|---|---|
| `ONBOARDING_VALIDATE_URL` | `http(s)://<onboarding-host>/auth/validate` — backend validation. |
| `VITE_ONBOARDING_URL` (or equiv) | Portal base — frontend redirects + logout. |

---

## 8. Security notes

- **Cookie:** `HttpOnly`, `SameSite=Lax`, `Secure` in prod. The browser never holds a
  token — only the opaque `session_id`.
- **`redirect` is allow-listed.** The portal only honors a post-login `redirect` whose
  origin is a known product (`CORS_ALLOWED_ORIGINS`), so it can't be an open redirect.
- **Email is verified by the IdP** for SSO; password/OTP verification is enforced by the
  platform. Products can trust `email_verified`.
- **Authorization stays in the product.** Never trust a role/tenant from the session —
  derive them from your own DB keyed on the mapped internal user.
- **Validation should be cached briefly** (seconds) per request-scope to avoid a
  round-trip on every call; invalidate on logout.

---

## 9. Integration checklist

- [ ] Cookie is shared (subdomain in prod / host-only localhost in dev).
- [ ] Frontend redirects unauthenticated users to `${ONBOARDING_URL}/auth?mode=signin&redirect=<url>`.
- [ ] Product renders **no** local login/signup page.
- [ ] Backend validates every protected request via `GET /auth/validate` (cookie forwarded).
- [ ] `sub → internal_user_id` upsert exists in the product's own DB, syncing email + names.
- [ ] Tenant/roles/permissions derived from the product's own data (not the session).
- [ ] Sign-out calls `${ONBOARDING_URL}/auth/logout` then lands on the portal.
- [ ] `ONBOARDING_VALIDATE_URL` reachable from the product backend (network/alias).
- [ ] (Prod) `COOKIE_DOMAIN` set so the cookie spans all product subdomains.

---

## 10. Endpoint appendix (onboarding service)

```
GET  /health, /ready
GET  /validate                         # alias of /auth/validate
GET  /auth/validate                    # ← product integration endpoint
GET  /auth/me                          # current user (portal/proxy)
POST /auth/register                    # portal sign-up
POST /auth/login                       # portal password sign-in
POST /auth/demo                        # "Try it now" shared demo account
GET  /auth/logout    POST /auth/logout # destroy central session
GET  /auth/clients                     # product registry (portal)
GET  /auth/sso/{provider}              # start Google/Microsoft SSO
GET  /auth/sso/callback/{provider}     # SSO return (Zitadel IDP intent)
POST /auth/otp/email/start | verify    # email verification
POST /auth/otp/mobile/start | verify   # mobile verification
POST /auth/login/otp/start | verify    # passwordless sign-in
POST /onb/session, /onb/event          # value-first funnel analytics
GET  /onb/state, /onb/entitlements
POST /onb/demo                         # demo / POC / contact requests
```

---

*Maintainer note: the platform is intentionally product-agnostic. If you find yourself
adding a product-specific concept (a product's internal id, tenant, or role) to the
onboarding service, stop — that belongs in the product. The platform's job ends at
handing over a verified, universal `sub`.*
