# Simplify Onboarding — Backend Implementation Plan

**Module:** `simplify-onboarding-service` (identity + value-first onboarding backend)
**Status:** ✅ Core built, runs against real Zitadel, container deployed locally
**Owner:** Platform / Auth
**Goal:** One account, every Simplify product. A single, branded sign-up / sign-in
experience (like Google Account) backed by the existing Zitadel instance, so a user
authenticates **once** and that session carries across all 8 Simplify products — sequenced
**value-first** (try → see result → register → verify → pay) per the CX design.

> Read first: [00_MASTER_PLAN.md](00_MASTER_PLAN.md) — the value-first journey, motions,
> activation, and module boundaries this backend implements.
> Companion doc: [SIMPLIFY_ONBOARDING_FRONTEND_PLAN.md](SIMPLIFY_ONBOARDING_FRONTEND_PLAN.md)

> **§1–16 = the SSO identity backbone. Part B (§17+) adds the value-first onboarding layer.**

---

## ✅ Implementation status — `simplify-onboarding-service` (Go, builds + vets clean)

**Built & verified against real Zitadel (`auth.simplifyaipro.com`, org `370544088589533187`):**
- ✅ Modular service (chi/zap/go-redis), mirrors docflow-auth conventions; Dockerfile + `docker-compose.yml` (Redis + service), curl healthcheck, runs healthy on `:8090`.
- ✅ **Zitadel identity provider** (production default): `Register` → `POST /v2/users/human` (Zitadel **sends the verification email**), `VerifyEmail`/`ResendEmail`, deferred phone verify, `Authenticate` (`/v2/sessions`), `Get`. Retry-aware. **Verified end-to-end**: register → emailed code → verify → `emailVerified=true` → password login.
- ✅ **Org-scoped duplicate check** — only blocks when the email exists in *this* org; instance-wide username collisions are auto-resolved with a unique username.
- ✅ **Verify-late**: register auto-creates the session; `emailVerified` flips on confirm. `/auth/me` **reconciles** verification state live from Zitadel (catches verification done in the Zitadel console / email link / another device).
- ✅ HttpOnly Redis sessions (sliding + absolute TTL); `/auth/me`, `/auth/logout`.
- ✅ **Product registry** (`/auth/clients`, `/onb/products/{key}/motion`), **visitor/funnel** endpoints (`/onb/session|event|state`), **entitlements** stub (`/onb/entitlements`).
- ✅ Dev identity provider (Redis + bcrypt, real logged codes — no hardcoded `123456`) for offline work.

**Pending (next phases):** true OIDC SSO bridge (`/authorize`+`/callback` so products are silent-SSO clients — §6/§9), federated Google/Azure SSO (currently a stub), Cerbos principal-attribute handoff (§21), SimplifyCore real entitlements, rate-limit/CSRF hardening, ECS/Kong/ALB/OTEL deploy (§12). DocFlow is currently connected at the **shared-account** level (same Zitadel org + launch link), not yet silent SSO.

---

## 1. The idea in one paragraph

Today each product (DocFlow, Drive, …) redirects users into Zitadel's hosted login
through its own BFF. We already have `docflow-auth` (Go, :8090) acting as a Zitadel
OIDC/PKCE proxy with custom password login, registration, SSO IdPs, and Redis sessions.
**Simplify Onboarding** generalizes that service into a *product-agnostic* central identity
authority living on its own domain (e.g. `id.simplifyaipro.com`). Every product becomes
an OIDC **client** of one shared Zitadel project inside the **global Simplify org**
(currently `SimplifyDocFlow`, renameable). The central portal owns the branded
signup/login/OTP/MFA/SSO UI and holds the **central IdP session**. Because that session
lives at the `id.` domain, the second, third … product a user opens gets logged in
**silently** via OIDC — that is the single sign-on.

```
┌─────────────┐   user not logged in    ┌────────────────────────────┐
│  Product A  │ ───── OIDC authorize ──► │   Simplify Onboarding  (id. domain)│
│ (DocFlow)   │ ◄──── code + session ─── │  simplify-onboarding-web (branded) │
└─────────────┘                          │  simplify-onboarding-service (Go)  │
┌─────────────┐   user ALREADY logged in │      ▲ central session      │
│  Product B  │ ───── OIDC authorize ──► │      │ (Zitadel cookie)      │
│ (Drive)     │ ◄── silent code (no UI)─ │      ▼                       │
└─────────────┘                          │        Zitadel (IdP)        │
                                         └────────────────────────────┘
```

---

## 2. Why this works with what we already have

| Existing piece | Reused as |
|---|---|
| `docflow-auth` OIDC/PKCE proxy ([oidc.go](../docflow-auth/internal/handler/oidc.go)) | Core of `simplify-onboarding-service` — fork & generalize |
| Custom password login (Zitadel Sessions API) | Email+password path of central portal |
| `POST /register` (Zitadel Management API) | Branded "Create account" path |
| SSO IdP routes (`IDP_<NAME>_ID`, [config.go](../docflow-auth/internal/config/config.go)) | "Continue with Google / Azure AD" |
| Redis session store (DB1, 30m TTL / 8h abs) | Central IdP session |
| `app_users` table (Zitadel sub → internal UUID) | Stays the global identity map |
| `tenant_members` decoupling ([ZITADEL_ORG_DECOUPLING.md](../ZITADEL_ORG_DECOUPLING.md)) | Org membership stays per-product/tenant, NOT tied to login |
| Kong gateway, Cloud Map, ECS staging | Same deploy substrate, new service + route |
| MailForge ([project] centralised email) | Email OTP + verification mails |

**Net new** vs. today: (a) the service must serve **many** OIDC clients, not one;
(b) email **and** mobile OTP verification as a first-class step; (c) a product/client
registry; (d) a branded portal domain; (e) single-logout fan-out.

---

## 3. Architecture decisions (resolve before coding)

| # | Decision | Recommended default | Notes |
|---|---|---|---|
| D1 | Central session location | **Zitadel-held OIDC session** at `id.` domain; products keep their own short BFF session | True SSO, single logout, no shared cookie domain needed |
| D2 | Is `simplify-onboarding-service` the OIDC **provider** or a **proxy** in front of Zitadel? | **Proxy/orchestrator** — Zitadel remains the OIDC provider; our service owns branded UX + OTP + registration | Avoids re-implementing an OAuth server |
| D3 | One Zitadel project, N apps, or N projects? | **One project "Simplify Suite", one OIDC app per product** | Shared roles/grants, simplest SSO |
| D4 | Mobile OTP provider | **Pluggable** (`OTP_SMS_PROVIDER`); start with one (e.g. Twilio/Vonage/local ID provider) | Indonesia numbers (+62) → pick a provider with good ID delivery |
| D5 | Verify mobile at signup or defer? | **Defer** (matches screenshot: "we'll verify it later, only when it's needed") | Email OTP required at signup; mobile stored, verified on demand |
| D6 | Account namespace | **One global Zitadel org** for all products | Rename `SimplifyDocFlow` → `Simplify` later |
| D7 | New repo or fork? | **New module dir `simplify-onboarding-service/`**, copy+generalize from `docflow-auth` | Keeps DocFlow stable during rollout |

---

## 4. Zitadel configuration (foundation — do first)

This is mostly console/Terraform/management-API config, not app code.

- [ ] **Org**: confirm the single global org (currently `SimplifyDocFlow`); plan rename to `Simplify`.
- [ ] **Project**: create project **"Simplify Suite"** (`PROJECT_GRANT_REQUIRED=false`, role check as needed).
- [ ] **Applications** (one OIDC app per product, PKCE / `code` flow, `offline_access`):
  - [ ] `simplify-docflow`  → redirect `https://<docflow>/auth/callback`
  - [ ] `simplify-legal`    → redirect `https://<legal>/auth/callback`
  - [ ] `simplify-drive`    → redirect `https://<drive>/auth/callback`
  - [ ] `simplify-onboarding-web`   → the portal itself (login UI session)
  - [ ] …one per remaining product (8 total)
- [ ] **IdPs** (org-level, shared by all apps): Google, Azure AD → exposes `IDP_GOOGLE_ID`, `IDP_AZURE_ID`.
- [ ] **Branding**: set org private-labeling (logo, colors) so any Zitadel-rendered surface matches; primary UX is our own portal.
- [ ] **Custom domain / known redirect**: register `id.<domain>` as the auth surface.
- [ ] **Actions / claims**: add an action to enrich tokens with `simplify_uid` (internal UUID) and product entitlements claim so each product can authorize on first callback.
- [ ] **Service user + PAT**: scoped PAT for the management API (user creation, OTP-less admin ops) → `ZITADEL_SERVICE_PAT`.
- [ ] Capture all IDs/secrets into **Secrets Manager** `simplify/onboarding-service` (mirror existing `docflow/*` pattern).

---

## 5. Service shape (`simplify-onboarding-service/`)

Fork `docflow-auth` layout (chi router, zap, config-from-env). Suggested tree:

```
simplify-onboarding-service/
  cmd/onboarding/main.go
  internal/
    config/config.go          # + product registry, OTP, SMS provider envs
    handler/
      oidc.go                 # /authorize-bridge, /callback (per-product redirect_uri)
      password.go             # POST /login (Zitadel Sessions API)
      register.go             # POST /register (Zitadel Mgmt API) + start email OTP
      otp.go                  # POST /otp/email/{start,verify}, /otp/mobile/{start,verify}
      sso.go                  # GET /sso/{provider} + /sso/callback/{provider}
      session.go              # GET /me, GET /logout (single-logout fan-out)
      clients.go              # GET /clients (product registry, public metadata)
      health.go
    otp/
      store.go                # Redis: code, attempts, resend cooldown, TTL
      email.go                # MailForge sender
      sms.go                  # provider interface + impls (twilio/vonage/...)
    clients/registry.go       # product → {client_id, redirect_uris, name, icon}
    session/session.go        # Redis session (reuse docflow-auth)
    userstore/userstore.go    # app_users map (reuse) — login NEVER sets tenant
    zitadel/client.go         # Sessions + Management API wrappers (reuse)
    middleware/...            # cors, request_id, rate_limit, logger, otel
    server/server.go
  Dockerfile
  ecs-task.json
  go.mod
```

> Keep the **ZITADEL_ORG_DECOUPLING** invariant: a *login* writes only the
> `app_users` identity map. Org/tenant membership is **never** derived from the
> Zitadel org and stays a per-product concern via `tenant_members`.

---

## 6. API surface

All under the gateway prefix `/auth/*` on the `id.` domain (Kong route). Browser never
sees a JWT; the portal holds an HttpOnly session cookie scoped to `id.`.

| Method | Path | Purpose |
|---|---|---|
| `GET`  | `/auth/authorize?client_id=&redirect_uri=&state=` | Entry from a product. Validates client against registry, starts PKCE to Zitadel, remembers product `return` target. |
| `GET`  | `/auth/callback` | Zitadel → us. Exchange code (server-to-server), upsert `app_users`, set central session, bounce back to the originating product's `redirect_uri` with the product's own code. |
| `POST` | `/auth/register` | Create account (Zitadel Mgmt API). Returns `verification_id`; triggers **email OTP**; stores mobile unverified. |
| `POST` | `/auth/otp/email/start` / `/verify` | 6-digit email OTP. `verify` marks email verified, then issues/continues the auth flow. |
| `POST` | `/auth/otp/mobile/start` / `/verify` | Deferred mobile OTP (on-demand). |
| `POST` | `/auth/login` | Email+password via Zitadel Sessions API → central session. |
| `GET`  | `/auth/sso/{provider}` | Start federated SSO (Google/Azure). |
| `GET`  | `/auth/sso/callback/{provider}` | Finish federated SSO. |
| `GET`  | `/auth/me` | Current user (from central session) for the portal. |
| `GET`  | `/auth/logout` | Destroy central session + Zitadel `end_session` + fan-out single-logout. |
| `GET`  | `/auth/clients` | Public product registry (names/icons) for the portal's "every product" panel. |
| `GET`  | `/health` / `/ready` | ECS health checks (curl-based, like docflow-api fix). |

### OTP contract (email, signup)

```
POST /auth/otp/email/start   { email }                 → { verification_id, resend_in }
POST /auth/otp/email/verify  { verification_id, code }  → { verified: true, next: "<redirect>" }
```
- Redis: `otp:email:<verification_id>` → `{code_hash, attempts, expires}`; TTL 10m.
- Max 5 attempts, resend cooldown 30s, lockout after N. Codes hashed, never logged.
- **Prototype parity:** the screenshot mock accepts any 6 digits / `123456`; production
  must validate real codes — gate prototype bypass behind `OTP_DEV_BYPASS=true` (staging only).

---

## 7. Product / client registry

A small config-driven table so adding a 9th product is config, not code.

- [ ] Define registry (start as JSON/env, graduate to a `sso_clients` DB table):

```jsonc
{
  "simplify-docflow": { "name": "Simplify DocFlow", "client_id": "…",
    "redirect_uris": ["https://docflow…/auth/callback"], "post_logout": ["https://docflow…/auth"],
    "icon": "docflow", "tagline": "Document workflows" },
  "simplify-legal":   { "name": "SimplifyLegal", "client_id": "…",
    "redirect_uris": ["https://legal…/auth/callback"], "icon": "legal",
    "tagline": "Legal chatbot, AI Lawyer, document review" }
}
```

- [ ] `redirect_uri` from a product **must** match a registered URI (open-redirect guard).
- [ ] `/auth/clients` returns only public fields (name, tagline, icon) for the portal.
- [ ] Optional `sso_clients` + `client_redirect_uris` tables if registry needs runtime edits.

---

## 8. Data model changes

Mostly reuse. New tables are small and additive (migrations dir already exists).

- [ ] `migrations/0XX_simplify_id.sql`
  - [ ] `sso_clients` (id, key, name, tagline, icon, created_at) — optional if config-only.
  - [ ] `client_redirect_uris` (client_key, uri) — optional.
  - [ ] `user_contacts` (user_id FK `app_users`, email_verified_at, phone, phone_verified_at) —
        tracks deferred mobile verification; or extend existing profile fields
        (see `017_add_zitadel_profile_fields.sql`).
- [ ] **No change** to `tenant_members` / tenant logic — keep decoupling intact.
- [ ] `app_users` stays the single global identity map across all products.

---

## 9. Single sign-on & single logout mechanics

- [ ] **SSO (login once):** all products point OIDC `authorize` at `id.<domain>`. Zitadel's
      own session cookie at that domain means the 2nd+ product gets a code with no UI —
      verify `prompt=none` / silent-auth works for product BFFs.
- [ ] **Session lifetime:** central session 30m idle / 8h absolute (reuse defaults); product
      BFF sessions independent and shorter — they re-silent-auth against center.
- [ ] **Single logout:** `/auth/logout` clears central session, calls Zitadel `end_session`,
      and (phase 2) fans out back-channel logout to each product's `/auth/backchannel-logout`.
- [ ] **Account switching:** preserve existing account-switch behavior
      ([ACCOUNT_SWITCH_FIX_PLAN.md](../ACCOUNT_SWITCH_FIX_PLAN.md)) at the central layer.

---

## 10. Security checklist

- [ ] PKCE (S256) on every product↔center and center↔Zitadel hop (already in `oidc.go`).
- [ ] `state` + `nonce` stored server-side in Redis (10m), one-time use.
- [ ] Strict `redirect_uri` allow-list per client (no open redirect).
- [ ] OTP: hashed codes, rate-limit + lockout, resend cooldown, constant-time compare, no PII in logs.
- [ ] Session cookie: HttpOnly, Secure, SameSite=Lax, scoped to `id.` host.
- [ ] CSRF protection on all `POST` form endpoints.
- [ ] Rate-limit `/login`, `/register`, `/otp/*` (reuse BFF rate_limit middleware).
- [ ] Brute-force / credential-stuffing protections (Zitadel lockout policy + our throttle).
- [ ] OTEL spans + audit log (login, register, otp_verify, logout) → existing Loki/Tempo stack.
- [ ] Secrets only in Secrets Manager; PAT least-privilege.

---

## 11. Config / env (new + reused)

```
PORT=8090
ZITADEL_ISSUER / ZITADEL_JWKS_URL / ZITADEL_EXTERNAL_URL / ZITADEL_INTERNAL_URL
ZITADEL_SERVICE_PAT / ZITADEL_ORG_ID
ID_PORTAL_URL=https://id.<domain>          # post-login default landing (the portal)
GATEWAY_BASE_URL=https://id.<domain>       # SSO callback base
REDIS_URL=…  (session DB1, otp DB2)
DATABASE_URL=…  (app_users via pgbouncer.docflow.local:6543)
INTERNAL_JWT_SECRET=…
CORS_ALLOWED_ORIGINS=https://id.<domain>,https://<each product>
IDP_GOOGLE_ID=… / IDP_AZURE_ID=…           # auto-discovered (existing pattern)
SSO_CLIENTS_JSON=…  or  SSO_CLIENTS_FILE=/etc/simplify/clients.json
OTP_EMAIL_FROM / MAILFORGE_BASE_URL / MAILFORGE_API_KEY
OTP_SMS_PROVIDER=twilio|vonage|… / OTP_SMS_* (provider creds)
OTP_TTL=10m / OTP_MAX_ATTEMPTS=5 / OTP_RESEND_COOLDOWN=30s
OTP_DEV_BYPASS=false                        # staging-only prototype bypass
OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector.docflow.local:4318
```

---

## 12. Deployment (mirror staging pattern)

- [ ] Dockerfile (multi-stage Go), curl-based ECS `healthCheck` (avoid the docflow-api hang class of bug).
- [ ] `ecs-task.json` → ECS service on `ap-southeast-3`, Cloud Map name `simplify-onboarding.docflow.local`.
- [ ] Kong route: `id.<domain>` → `/auth/*` → `simplify-onboarding.docflow.local:8090`; serve portal SPA on `/`.
- [ ] ALB/HTTPS: add `id.<domain>` to cert + listener rules (mirror
      [project] staging-https-domain).
- [ ] Wire OTEL endpoint + Loki/Tempo (mirror staging otel).
- [ ] DB via `pgbouncer.docflow.local:6543` (mirror staging db mesh).
- [ ] Persist live task def back to repo `ecs-task.json [skip ci]` per house style.

---

## 13. Migration / rollout (zero-downtime)

1. **Phase 0** — Zitadel config (§4) + service skeleton, deploy to staging behind `id-staging.<domain>`.
2. **Phase 1** — Make **DocFlow** the first client of Simplify Onboarding (point its BFF OIDC at `id.`).
   DocFlow already uses this flow → lowest-risk pilot. Verify login, SSO session, logout.
3. **Phase 2** — Onboard a second product (Legal or Drive) → prove silent SSO across two products.
4. **Phase 3** — Roll remaining products; enable single-logout fan-out; rename Zitadel org `SimplifyDocFlow`→`Simplify`.
5. **Phase 4** — Decommission per-product hosted-login redirects; Simplify Onboarding is the only auth surface.

---

## 14. Testing

- [ ] Unit: OTP store (attempts/expiry/lockout), redirect_uri allow-list, registry validation.
- [ ] Integration: full PKCE round-trip product→center→Zitadel→center→product.
- [ ] SSO: log into product A, open product B → no second login prompt.
- [ ] SLO: logout from one product kills session everywhere.
- [ ] Negative: tampered `redirect_uri`, replayed `state`, expired/﻿wrong OTP, OTP brute-force lockout.
- [ ] Federated: Google + Azure AD sign-in create the same `app_users` identity.
- [ ] Load/health: ECS health check flaps test; pgbouncer half-open resilience (keepalives).

---

## 15. Open questions for the manager / user

- [ ] Final auth domain? (`id.simplifyaipro.com` vs `account.simplify…`)
- [ ] Which SMS provider for +62 mobile OTP delivery, and what budget?
- [ ] Confirm the canonical list of all 8 products + their redirect URIs.
- [ ] Rename the global Zitadel org now or after pilot? (`SimplifyDocFlow` → `Simplify`)
- [ ] MFA/passkeys: launch with email OTP only, add WebAuthn passkeys in phase 2? (screenshot lists "MFA & passkeys")
- [ ] Do non-DocFlow products already exist as deployable apps, or is this greenfield for them?

---

## 16. Task summary (high level)

- [ ] §4 Zitadel project + per-product OIDC apps + IdPs + branding + PAT
- [x] §5 Scaffold `simplify-onboarding-service/` from `docflow-auth`
- [ ] §6 OIDC bridge (`/authorize`,`/callback`) multi-client
- [x] §6/§7 Product registry + redirect_uri allow-list
- [x] §6 Email OTP (start/verify) via MailForge
- [x] §6 Deferred mobile OTP + SMS provider abstraction
- [x] §6 Password login + register + federated SSO (port from docflow-auth)
- [ ] §9 SSO silent-auth + single-logout fan-out
- [ ] §8 Migrations (sso_clients / user_contacts)
- [ ] §10 Security hardening pass
- [ ] §12 Dockerfile + ECS + Kong + ALB + OTEL deploy
- [ ] §13 Phase-1 DocFlow pilot → multi-product rollout

---
---

# Part B — Value-first onboarding layer (CX doc)

> Part A above gives a working "log in once across products". Part B sequences the
> *acquisition* journey so users **see value before any gate**. The ordering principle:
> `See value → small commitment → more value → register → verify → pay`.

## 17. Anonymous identity & intent capture

Before anyone registers we still want to know *who is in the funnel* (company-level) and
*what they tried*. The service issues an anonymous session and records intent.

| Endpoint | Purpose |
|---|---|
| `POST /onb/session` | Mint an anonymous `visitor_id` (HttpOnly cookie). Attach reverse-IP company (async), source/campaign, geo, device. |
| `POST /onb/event` | Record funnel events: `viewed_product`, `tried_action`, `saw_result`, `hit_value_wall`, `registered`, `verified`. Forwarded to analytics (PostHog) + CRM for identified companies. |
| `GET  /onb/state` | Return the visitor's current stage + last intent so the frontend can resume the journey and pick the right CTA. |

- [ ] `visitors` (Redis + warehouse sink) — `visitor_id`, company (reverse-IP), utm/source, geo, device, first_seen.
- [ ] **Reverse-IP / company-ID** provider behind an interface (`COMPANY_ID_PROVIDER` — Clearbit Reveal / RB2B / Albacross / Leadfeeder). No-op if unset.
- [ ] **Analytics** sink behind an interface (`ANALYTICS_PROVIDER` — PostHog/Amplitude/Mixpanel).
- [ ] On registration, **link** `visitor_id` → `app_users.id` so pre-register intent attaches to the identity (and flows to CRM as a qualified lead).

> Privacy: anonymous analytics + company identification is **company-level**, not personal.
> Personal identity is captured only at registration. Consent checkbox is opt-in (§11 master).

## 18. The value wall (deferred registration)

A product lets a self-serve user run **one scoped action without logging in**, then gates
the *result-keeping / continuation* behind registration.

```
tried_action (anonymous) → saw_result (ephemeral, server-held)
  → hit_value_wall → POST /onb/register → claim the result into the new account
```

- [ ] `POST /onb/trial/run` — execute the one scoped trial action for the product (proxied to the product/AI service) under the anonymous session; persist the result ephemerally (Redis, short TTL) keyed by `visitor_id`.
- [ ] `POST /onb/register` — create the Zitadel user (Mgmt API), link `visitor_id`, then **claim** the ephemeral result into the user's workspace so nothing is lost at the wall.
- [ ] Capture at the wall: **first/last name, work email, company, job title, mobile** (matches the updated mock — note Company + Job Title are now on the create-account form), consent (opt-in).
- [ ] Self-serve registration **never** blocks on verification — see §19.

## 19. Verification: capture early, verify late (dual OTP)

The updated mock shows **email + mobile** ("we'll send a one-time code to both"). The CX doc
says **verify as late as possible**. Reconcile: capture both at registration, **require email
verification only before the first sensitive action** (save / pay / real-data upload), and
verify **mobile on demand**.

| Endpoint | Purpose |
|---|---|
| `POST /onb/otp/email/start` · `/verify` | 6-digit email OTP via MailForge. |
| `POST /onb/otp/mobile/start` · `/verify` | 6-digit SMS OTP via `OTP_SMS_PROVIDER` (+62 capable). |
| `GET  /onb/verification/status` | `{ email_verified, mobile_verified }` so products can gate sensitive actions. |

- [ ] **Sensitive-action gate** helper the products call: `requireVerified(user, action)` → for `save|pay|real_upload` demand `email_verified`; for high-risk (KYC, real applicant data KTP/SLIK/BPKB) demand mobile + data-handling terms.
- [ ] Retail path stays **light**: email confirmation only, no domain check, no docs (CX doc).
- [ ] OTP store: hashed codes, ≤5 attempts, 30s resend cooldown, lockout, 10m TTL, constant-time compare, never logged. `OTP_DEV_BYPASS=true` for staging only.

## 20. Motion routing & entitlements (read SimplifyCore)

The product determines allowed user types and the motion drives the CTA (master §2, §5).

- [ ] `GET /onb/products/{key}/motion` — returns `{ allowed_user_types, asks_user_type, primary_cta, trial_scope, data_residency }` from the product registry (§7) so the frontend renders the "How will you use this?" split and the right CTA.
- [ ] **Entitlement read** from SimplifyCore (we never store billing): `GET /onb/entitlements` → `{ org, products:[{key, plan, trial_status, credits}] }`. Used to decide value-wall vs trial vs locked, and to render contextual upgrade.
- [ ] `SIMPLIFYCORE_BASE_URL` + service token; **stub** mode (`SIMPLIFYCORE_STUB=true`) returns a fake free-trial entitlement until Core ships.

## 21. Cerbos handoff (authorization)

Authentication = Zitadel; authorization = Cerbos (existing `cerbos/`,
[CERBOS_RBAC_SYSTEM_PLAN.md](../CERBOS_RBAC_SYSTEM_PLAN.md)). The onboarding service does **not**
make per-resource decisions — it assembles the **principal attributes** products pass to Cerbos.

- [ ] On session/validate, surface principal attrs: suite roles + per-product roles (Zitadel claims), `tenant_id` (DocFlow UUID, decoupled), entitlement/plan/credits (SimplifyCore), and scoped metadata (e.g. Hiring `assigned_jobs`, Credit `jurisdiction`).
- [ ] Keep the [ZITADEL_ORG_DECOUPLING](../ZITADEL_ORG_DECOUPLING.md) invariant: login writes only `app_users`; membership via `tenant_members`; never derive tenant from the Zitadel org claim.

## 22. Provisioning & user management (enterprise)

Supports the "from sold to live" handoff (master §9). Mostly Zitadel Management API.

- [ ] `POST /onb/provision/org` (internal, sales/SimplifyCore-triggered) — create Zitadel org + first user as `ORG_OWNER` (Org Admin); grant Suite project + purchased product roles.
- [ ] `POST /onb/org/invite` (Org Admin) — invite users with role + product-access assignment; email via MailForge; invite-accept sets password / auth method.
- [ ] **Verified self-signup** path (smaller deals): work-email domain verified → workspace pending plan assignment (handed to SimplifyCore).
- [ ] **Vendor-as-collaborator:** generate scoped vendor invite code/link for a job → external grant scopes to assigned jobs only (Hiring).

## 23. Behavioral email triggers (stall recovery)

Behaviour-triggered, not calendar (master §6). The service emits trigger events; MailForge
templates/sends.

- [ ] Emit on: `signed_up_no_action`, `saw_result_no_return`, `hit_credit_limit`, `enterprise_demo_no_show`.
- [ ] Each maps to a MailForge template that reinforces *unfinished value* (not generic education); upgrade prompts are card-style.

## 24. Part B config (additions)

```
COMPANY_ID_PROVIDER=clearbit|rb2b|albacross|leadfeeder|none
ANALYTICS_PROVIDER=posthog|amplitude|mixpanel|none   ANALYTICS_KEY=…
CRM_WEBHOOK_URL=…                                     # identified-lead sync
SIMPLIFYCORE_BASE_URL=…  SIMPLIFYCORE_TOKEN=…  SIMPLIFYCORE_STUB=true
OTP_SMS_PROVIDER=…  OTP_SMS_*                         # +62 capable
ONB_TRIAL_RESULT_TTL=30m                              # ephemeral pre-register result
PRODUCT_REGISTRY_FILE=/etc/simplify/products.json     # motion + trial scope + residency
```

## 25. Part B task summary

- [ ] §17 Anonymous session + intent events + reverse-IP/analytics/CRM interfaces
- [ ] §18 Value-wall: scoped trial run + ephemeral result claim on register
- [x] §18 Register captures name/email/company/job-title/mobile + opt-in consent
- [x] §19 Dual OTP (email required pre-sensitive-action; mobile on demand) + sensitive-action gate
- [x] §20 Product motion endpoint + SimplifyCore entitlement read (with stub)
- [ ] §21 Cerbos principal-attribute assembly (roles + entitlement + scoped metadata)
- [ ] §22 Provisioning + org invite + vendor scoped invite
- [ ] §23 Behavioral-email trigger emission
- [ ] Wire Part B into the phased rollout (master §13: P2 value-first, P4 activation, P5 enterprise, P6 Core/Cerbos)
