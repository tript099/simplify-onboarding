# simplify-onboarding-service

Identity + value-first onboarding backend for the Simplify suite (Go). Owns
registration, email/mobile OTP, sessions, the product registry, anonymous funnel
capture and entitlement reads — the spine single sign-on is built on.

See the plans: [00_MASTER_PLAN.md](../00_MASTER_PLAN.md) ·
[Backend plan](../SIMPLIFY_ONBOARDING_BACKEND_PLAN.md).

## Run

```bash
cp .env.example .env          # defaults are dev-friendly
docker run -d --rm -p 6379:6379 redis:7-alpine   # or point REDIS_URL at any Redis
go run ./cmd/onboarding       # listens on :8090
```

### Production (default) — Zitadel

`IDENTITY_PROVIDER=zitadel` (the default). **Zitadel stores passwords and sends the
verification email** — exactly like docflow-auth. Wire it from the repo-root `.env`:

```bash
docker compose --env-file ../../.env up -d --build
```

That gives the container `ZITADEL_ISSUER`, `ZITADEL_SERVICE_PAT`, `ZITADEL_ORG_ID`.
Registration creates a real Zitadel user (`POST /v2/users/human`) and Zitadel emails
the 6-digit code; `/auth/otp/email/verify` confirms it via `POST /v2/users/{id}/email/verify`.

For **local testing without reading an inbox**, set `DEBUG_RETURN_CODE=true` — Zitadel
returns the code in the API response (and the UI shows it). Never enable in production.

### Offline dev — no Zitadel

`IDENTITY_PROVIDER=dev` stores users in Redis (bcrypt) and **logs** the real generated
code (no hardcoded `123456`). For local work with no IdP.

Point the frontend at either with `VITE_USE_MOCK=false` (it calls `/auth/*` same-origin).

## Architecture (modular by design)

```
cmd/onboarding         entrypoint (graceful shutdown)
internal/
  config               env → typed Config
  httpx                JSON + error-envelope helpers
  session              Redis HttpOnly sessions (sliding TTL + absolute max)
  middleware           X-Internal-Secret guard
  identity             Provider interface  ├─ dev (Redis + bcrypt, works now)
                                           └─ zitadel (scaffold; port from docflow-auth)
  otp                  hashed codes, attempt limits, resend cooldown
                       Sender interface     ├─ log (dev)  └─ mailforge (email)
  catalog              the 8 products + motion + trial scope (+ JSON override)
  visitor              anonymous identity + funnel events ("capture early, verify late")
  entitlements         SimplifyCore client (stub → free trial until Core ships)
  handler              HTTP surface (health, auth, otp, onboarding)
  server               dependency wiring + chi router
```

Every collaborator sits behind an interface, so swapping the dev identity backend
for Zitadel, the log sender for MailForge/SMS, or the entitlement stub for real
SimplifyCore is a one-line change in `server.New` — no handler touches.

## API

| Method | Path | Purpose |
|---|---|---|
| GET  | `/health` `/ready` | probes |
| POST | `/auth/register` | create account → start email OTP → `{verificationId,email}` |
| POST | `/auth/login` | email+password → session → `{next}` |
| GET  | `/auth/me` | current user from session |
| GET  | `/auth/logout` | destroy session |
| GET  | `/auth/clients` | product registry |
| GET  | `/auth/sso/{provider}` | federated SSO start (stub → Zitadel) |
| POST | `/auth/otp/email/start` · `/verify` | email OTP (verify mints the session) |
| POST | `/auth/otp/mobile/start` · `/verify` | deferred mobile OTP (signed-in) |
| POST | `/onb/session` | mint anonymous visitor id |
| POST | `/onb/event` | record a funnel event |
| GET  | `/onb/state` | visitor's funnel position |
| GET  | `/onb/products/{key}/motion` | motion + primary CTA for a product |
| GET  | `/onb/entitlements` | plan/entitlement snapshot (SimplifyCore) |

## Verified end-to-end

`register → otp verify (123456) → session → /me`, duplicate-email `409`, and
wrong-password `401` all pass against a local Redis. `go build ./...` and
`go vet ./...` are clean.

## Production path

- `IDENTITY_PROVIDER=zitadel` → port the Management/Sessions API calls from
  `docflow-auth` into `internal/identity/zitadel.go` (the call list is documented there).
- Set `MAILFORGE_BASE_URL` for real email; plug an SMS provider into `internal/otp`.
- Set `SIMPLIFYCORE_BASE_URL` (and `SIMPLIFYCORE_STUB=false`) for real entitlements.
- Deploy via the `Dockerfile` (curl healthcheck) on ECS, fronted by Kong at the `id.` domain.
