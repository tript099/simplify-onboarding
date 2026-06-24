# Simplify Onboarding — Frontend Implementation Plan

**Module:** `simplify-onboarding-web` (branded onboarding portal + homepage + product pages)
**Status:** Planning
**Owner:** Platform / Frontend
**Goal:** Build the "One account, every product" experience shown in the mocks — a single,
branded surface that routes visitors **by problem**, lets them **see value before any gate**,
then registers, verifies, and SSO-carries them across all 8 Simplify products.

> Read first: [00_MASTER_PLAN.md](00_MASTER_PLAN.md) — value-first journey, motions, CTA
> framework, activation.
> Companion doc: [SIMPLIFY_ONBOARDING_BACKEND_PLAN.md](SIMPLIFY_ONBOARDING_BACKEND_PLAN.md)
> (Part A §6 = auth APIs; Part B §17–24 = onboarding APIs).

> **§1–12 = the branded auth portal (the two account mocks; still correct).**

---

## ✅ Implementation status — `simplify-onboarding-web` (builds clean, runs on :3100, wired to the live backend)

**Built & working** (against the live `simplify-onboarding-service`; also runs standalone with the mock):
- ✅ Vite + React + TS + Tailwind scaffold; dark-first design system + light **theme toggle**; **Lenis smooth scroll**; **skeleton loading**; framer-motion micro-interactions; real **Simplify logo**.
- ✅ **AuthLayout + BrandPanel + ProductChip** (driven by `?product=`/`?client_id=`).
- ✅ **Create account** — Google **+ Microsoft** SSO buttons, work-email "domain verified" check, company prefill, **international PhoneField with searchable country picker + real flags**, **Display name (optional)**, **Confirm password**, password strength, opt-in consent gate → register.
- ✅ **Verify email** — 6-box OTP that accepts Zitadel's **uppercase alphanumeric** codes (paste/arrow/autofill), resend cooldown, error shake.
- ✅ **Sign in** — email/password + SSO buttons.
- ✅ **Logged-in account state** — `useAuth` over `/auth/me`; header **AccountMenu** (avatar, name, email, verified badge, Sign out); personalized home greeting; state refreshes after login/verify; sign-out clears it.
- ✅ **Homepage problem-router** + **product landing page** (motion split, CTA-by-motion, trial chip, data residency).
- ✅ **Cross-product launch** — signed-in users **Open SimplifyDocFlow** from the home card / product page (`VITE_DOCFLOW_URL`); shared Zitadel account.
- ✅ **Live wiring** — Vite proxy `/auth`+`/onb` → backend; `VITE_USE_MOCK=false` exercises the real service.

**Not yet built** (later phases): SSO callback screen, deferred mobile-OTP screen, try-before-register value wall, Dockerfile/nginx/ECS deploy, automated tests.

> Per-line ✅ ticks below mark what's done. Open `- [ ]` items are pending.

---

## 1. What we're building (from the mocks)

Two screens were provided:

**Screen 1 — Create account / Sign in (split layout):**
- Left brand panel: "One account, every product." + the active product chip
  ("SimplifyLegal · Legal chatbot, AI Lawyer, document review") + feature bullets
  (MFA & passkeys, Email + mobile OTP, Federated SSO (Google, Azure AD), SOC 2).
- Right form panel: tab switch **Create account / Sign in**, "Continue with Google",
  divider "or with email", First/Last name, **Work email** (with live "Company domain
  verified" check), Mobile number, Password, privacy opt-in checkbox (unchecked by
  default), primary CTA **"Verify email & mobile →"**.

**Screen 2 — Verify email (OTP):**
- 🔐 card, "Verify your email", "We sent a 6-digit code to deep@simplifyai.id".
- Prototype hint line ("any 6 digits will verify. Try 123456") — **dev/staging only**.
- 6-box OTP input, **Resend code**, **Verify & continue →**.
- Footnote: "Your mobile (+62…) stays on file — we'll verify it later, only when needed."
  → confirms **deferred mobile verification** (matches backend D5).

The portal is a small, standalone SPA served at the auth domain (e.g. `id.<domain>`).

---

## 2. Stack & reuse decision

Match the existing `docflow-frontend` stack so we share components and design tokens:

| Concern | Choice | Why |
|---|---|---|
| Framework | **React + Vite + TypeScript** | Same as [docflow-frontend](../docflow-frontend/package.json) |
| UI kit | **shadcn/ui + Radix + Tailwind** | Already in DocFlow (`components.json`, `tailwind.config.ts`) |
| Forms | **react-hook-form + zod** (`@hookform/resolvers`) | Already a dependency |
| Data | **@tanstack/react-query** | Already a dependency |
| Build/serve | Vite build → static SPA behind Kong (nginx, like DocFlow) | Same deploy pattern |
| Repo | **New dir `simplify-onboarding-web/`** | Standalone; can lift shared UI from DocFlow |

> Decision: **separate SPA**, not a route inside DocFlow — the portal must serve *all*
> products and live on the neutral `id.` domain. Copy the shadcn primitives + Tailwind
> config from `docflow-frontend` to keep visual parity, then theme as "Simplify".

---

## 3. Auth model the frontend follows

The browser **never** holds a JWT (same rule as
[zitadelAuth.ts](../docflow-frontend/src/services/zitadelAuth.ts)). The portal talks only to
the Simplify Onboarding backend over same-origin `/auth/*` and relies on an HttpOnly session
cookie. Flow:

```
Product → window.location = id.<domain>/auth/authorize?client_id&redirect_uri&state
  ├─ user logged in already → backend silently bounces back to product (no UI)
  └─ not logged in → portal SPA renders:
        Create account ──► POST /auth/register ──► OTP screen ──► POST /auth/otp/email/verify
        Sign in ─────────► POST /auth/login ─────────────────────────────────────────────┐
        Continue w/ Google ► GET /auth/sso/google (full redirect)                          │
                                                                                          ▼
                                              backend sets central session, 302 → product redirect_uri
```

---

## 4. Project structure (`simplify-onboarding-web/`)

```
simplify-onboarding-web/
  index.html
  vite.config.ts
  tailwind.config.ts            # lifted from docflow-frontend, re-themed
  components.json               # shadcn config
  nginx.conf  server.js  Dockerfile   # same serve pattern as docflow-frontend
  src/
    main.tsx  App.tsx
    routes/
      AuthLayout.tsx            # split brand panel + form panel
      CreateAccount.tsx         # Screen 1 (create tab)
      SignIn.tsx                # Screen 1 (sign-in tab)
      VerifyEmail.tsx           # Screen 2 (email OTP)
      VerifyMobile.tsx          # deferred mobile OTP (on-demand)
      SsoCallback.tsx           # post-redirect spinner / error
    components/
      BrandPanel.tsx            # "One account, every product" + product chip + bullets
      ProductChip.tsx           # driven by ?client_id → /auth/clients lookup
      GoogleButton.tsx
      OtpInput.tsx              # 6-box segmented input (paste, arrow nav, autofill)
      PasswordField.tsx         # show/hide + strength
      WorkEmailField.tsx        # live "Company domain verified" check
      PhoneField.tsx            # intl, defaults +62
      Field.tsx Divider.tsx Spinner.tsx
    lib/
      api.ts                    # fetch wrapper (credentials: 'include')
      authClient.ts             # register/login/otp/sso/me/logout calls
      clients.ts                # GET /auth/clients (product registry)
      validation.ts             # zod schemas
      query.ts                  # react-query setup
    styles/  theme.css tokens.css
    ui/                         # shadcn primitives copied from docflow-frontend
  src/components/ui/...
```

---

## 5. Screen-by-screen build

### 5.1 AuthLayout + BrandPanel
- [x] Two-column responsive layout (stacks on mobile). Dark theme matching mock
      (deep navy gradient, blue accent `#3b82f6`-ish).
- [x] `BrandPanel`: headline "One account," / "every product." (accent on 2nd line),
      subcopy, **ProductChip** (active product), feature bullet list.
- [x] `ProductChip` reads `?client_id` from the URL, fetches `/auth/clients`, shows the
      matching product name + tagline + icon. Falls back to generic "Simplify".

### 5.2 Create account (Screen 1)
- [x] Tab switch Create / Sign in (Radix Tabs), preserves `client_id`+`redirect_uri`+`state` across tabs.
- [x] `GoogleButton` → `window.location = /auth/sso/google?...` (full redirect).
- [x] Divider "or with email".
- [x] Fields (matching the updated mock): First name, Last name, **Work email**,
      Mobile number, **Company**, **Job title**, Password. (Company/Job title double as the
      enterprise qualification fields — kept to a few, smart-defaulted; see master §3 / CX
      "+ company profile".)
- [x] `WorkEmailField`: debounced check → shows green "✓ Company domain verified" when the
      email domain is an allowed company domain (backend `/auth/email/check` or client-side allow-list).
- [x] Prefill **Company** from the verified work-email domain when possible.
- [x] `PhoneField`: default country **+62**, E.164 validation.
- [x] `PasswordField`: min policy mirroring Zitadel's password policy; show/hide; strength meter.
- [x] Privacy opt-in checkbox — **unchecked by default** (mock explicitly notes this); blocks submit until checked.
- [x] CTA "Verify email & mobile →": `POST /auth/register` → on success store
      `verification_id`, route to **VerifyEmail**.
- [x] Inline field errors via zod + server error mapping (email taken, weak password, etc.).

### 5.3 Verify email (Screen 2)
- [x] `OtpInput`: 6 segmented boxes — autofocus, arrow/backspace nav, paste-fills-all,
      mobile numeric keyboard, `autocomplete="one-time-code"` for OS autofill.
- [x] Shows target email ("We sent a 6-digit code to deep@simplifyai.id").
- [x] **Resend code**: `POST /auth/otp/email/start`, 30s cooldown timer, disabled while counting.
- [x] "Verify & continue →": `POST /auth/otp/email/verify` → on `verified:true` follow
      `next` redirect (back to product `redirect_uri`).
- [x] Footnote about deferred mobile verification.
- [x] Dev hint banner ("any 6 digits / 123456") **only** when `VITE_OTP_DEV_HINT=true` (staging).
- [x] Back link → returns to Screen 1 keeping form state.
- [x] Error states: wrong code (shake + count remaining attempts), expired (prompt resend), locked out.

### 5.4 Sign in
- [x] Email + password → `POST /auth/login` → follow redirect. "Continue with Google" reused.
- [x] "Forgot password?" → Zitadel reset flow (or `/auth/password/reset`).
- [x] Friendly mapping for invalid credentials / locked account.

### 5.5 SSO + callback
- [ ] `SsoCallback.tsx`: spinner while backend finishes; on error show retry to `/auth/authorize`.

### 5.6 Deferred mobile verify (on-demand, phase 2)
- [ ] `VerifyMobile.tsx` reuses `OtpInput`; triggered later by a product when a sensitive
      action needs a verified phone (`/auth/otp/mobile/start|verify`).

---

## 6. API client (`lib/authClient.ts`)

Thin wrappers, all `credentials: 'include'`, same-origin:

```ts
register(body)            -> POST /auth/register        // { verification_id }
login(email, password)    -> POST /auth/login           // 302 / { next }
otpEmailStart(email)      -> POST /auth/otp/email/start  // { resend_in }
otpEmailVerify(id, code)  -> POST /auth/otp/email/verify // { verified, next }
otpMobileStart/Verify     -> POST /auth/otp/mobile/*
me()                      -> GET  /auth/me               // current user or 401
logout()                  -> GET  /auth/logout
clients()                 -> GET  /auth/clients          // product registry for BrandPanel
ssoUrl(provider)          -> string  // /auth/sso/{provider}?<preserved params>
```

- [ ] Preserve `client_id` / `redirect_uri` / `state` through every navigation (URL or sessionStorage).
- [ ] Central error mapper: server `code` → user-friendly copy.
- [ ] React-query for `clients()` + `me()`; mutations for the rest.

---

## 7. Design system / theming

- [x] Lift `tailwind.config.ts`, `components.json`, and `src/components/ui/*` from
      `docflow-frontend`; re-skin tokens to **Simplify** brand (logo lightning mark, blue accent).
- [x] `tokens.css`: dark navy surfaces, blue primary, green "verified" accent, generous radius (mock uses ~16px cards).
- [x] Accessibility: focus rings, labels, `aria-*` on OTP boxes, color-contrast AA, reduced-motion.
- [x] Responsive: split layout → single column under `md`; OTP boxes shrink gracefully.
- [ ] Product-aware theming (optional phase 2): accent per product from `/auth/clients`.

---

## 8. Config / env

```
VITE_AUTH_BASE=/auth                 # same-origin via Kong
VITE_OTP_DEV_HINT=false              # show prototype "123456" banner (staging only)
VITE_DEFAULT_COUNTRY=ID              # phone field default +62
VITE_BRAND=Simplify
```
Served behind Kong at `id.<domain>`; SPA fallback to `index.html` (nginx, like DocFlow).

---

## 9. Deployment (mirror docflow-frontend)

- [ ] Vite build → static; Dockerfile + nginx.conf + server.js copied from `docflow-frontend`.
- [ ] Kong route: `id.<domain>/` → SPA, `id.<domain>/auth/*` → `simplify-onboarding.docflow.local:8090`.
- [ ] ALB/HTTPS cert includes `id.<domain>`.
- [ ] ECS service on `ap-southeast-3`; persist `ecs-task.json` per house style.

---

## 10. Build order (suggested PRs)

1. **PR1** — Scaffold `simplify-onboarding-web/`, lift shadcn/Tailwind from DocFlow, AuthLayout + BrandPanel + ProductChip (static).
2. **PR2** — Create account form (validation, Google button, work-email check, phone, privacy gate) → `POST /auth/register`.
3. **PR3** — OtpInput + VerifyEmail screen + resend cooldown → `POST /auth/otp/email/*`.
4. **PR4** — Sign in + SSO buttons + SsoCallback; full product→portal→product round-trip on staging (DocFlow as first client).
5. **PR5** — Theming polish, a11y pass, responsive, error states, dev-hint gating.
6. **PR6** (phase 2) — Deferred mobile verify, passkeys/WebAuthn, per-product theming.

---

## 11. Testing

- [ ] Component: OtpInput (paste, autofill, backspace, attempts), WorkEmailField debounce, privacy gate blocks submit.
- [ ] Form validation: zod schemas (email, phone E.164, password policy).
- [ ] E2E (Playwright): create→OTP→redirect; sign-in→redirect; Google SSO; resend cooldown; wrong/expired OTP.
- [ ] SSO UX: second product opens with **no** login prompt (silent).
- [ ] Visual regression against the two provided mocks.
- [ ] a11y audit (axe) on both screens.

---

## 12. Open questions (shared with backend plan §15)

- [ ] Final auth domain + brand name on the portal ("Simplify" vs product-specific).
- [ ] Is "Company domain verified" a real allow-list check (B2B only) or cosmetic? Source of allowed domains?
- [ ] Launch with passkeys/MFA UI or defer to phase 2 (mock lists them as features)?
- [ ] Exact product list + icons + taglines for the BrandPanel/ProductChip.
- [ ] Password policy values (to mirror Zitadel) for the strength meter.
