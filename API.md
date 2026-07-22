# Simplify Onboarding Service — API reference

The onboarding backend is **Go (chi router)**, so it doesn't ship FastAPI's `/docs` by
default. It now serves an equivalent:

| What | URL (local) | URL (deployed) |
|---|---|---|
| **Swagger UI** (interactive) | http://localhost:8090/docs | https://onboarding.simplifyai.id/docs |
| **OpenAPI 3 spec** (YAML) | http://localhost:8090/openapi.yaml | https://onboarding.simplifyai.id/openapi.yaml |

> Source of truth: [`internal/handler/openapi.yaml`](simplify-onboarding-service/internal/handler/openapi.yaml)
> (embedded via `go:embed`), served by [`internal/handler/docs.go`](simplify-onboarding-service/internal/handler/docs.go).
> Import the spec URL into Postman/Insomnia to get a ready-made collection.

## Auth model

- The browser holds only an opaque **`session_id` cookie** (HttpOnly). Endpoints marked
  🔒 **cookie** require it.
- `POST /sms/zitadel` is authenticated by the **`ZITADEL-Signature`** header (HMAC).
- `POST /onb/demo/{id}/schedule` is internal — guarded by **`X-Internal-Secret`**.
- Error envelope everywhere: `{ "code": "...", "message": "..." }`.
- `debugCode` fields appear **only** in dev/testing (`DEBUG_RETURN_CODE`), never in prod.

---

## Endpoints (30)

### Health
| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/health` | — | Liveness probe → `{ ok, service }` |
| GET | `/ready` | — | Readiness probe → `{ ready }` |

### Session validation (products / Kong)
| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/validate` | 🔒 cookie | Resolve session → `{ valid, sub, email, … }` (401 `{valid:false}`) |
| GET | `/auth/validate` | 🔒 cookie | Alias of `/validate` |

### Auth
| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/auth/register` | — | Create account + sign in (sets cookie); emails a code. `409` email taken |
| POST | `/auth/login` | — | Email + password → session. `401` bad credentials |
| POST | `/auth/demo` | — | "Try it now" shared demo session. `404` if demo disabled |
| GET | `/auth/me` | 🔒 cookie | Current user (reconciles verification flags) |
| GET | `/auth/subscriptions` | 🔒 cookie | Per-product plan, keyed by product key (fail-soft empty) |
| GET | `/auth/logout` | — | Destroy session, clear cookie |
| POST | `/auth/password/forgot` | — | Email a reset link (always 200 — no enumeration) |
| POST | `/auth/password/reset` | — | Complete reset via emailed `userID`+`code`+`newPassword` |
| GET | `/auth/clients` | — | Public product registry (homepage cards) |

### SSO (federated — browser redirects)
| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/auth/sso/{provider}` | — | Begin Google/Microsoft SSO → 302 to IdP (`?redirect=`, `?display=popup`) |
| GET | `/auth/sso/callback/{provider}` | — | Complete IDP intent → 302 back to product (sets cookie) |

### OTP / verification
| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/auth/otp/email/start` | — | Resend email verification code |
| POST | `/auth/otp/email/verify` | — | Confirm email code → `{ verified, next }` |
| POST | `/auth/otp/mobile/start` | 🔒 cookie | Send SMS code to signed-in user |
| POST | `/auth/otp/mobile/verify` | 🔒 cookie | Confirm SMS code (own phone; sends only `code`) |
| POST | `/auth/login/otp/start` | — | Passwordless sign-in — request code (by email/phone) |
| POST | `/auth/login/otp/verify` | — | Passwordless sign-in — exchange code for session |

### Catalog & onboarding funnel
| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/onb/session` | — | Mint/return anonymous `visitorId` (sets `visitor_id` cookie) |
| POST | `/onb/event` | — | Record funnel event `{ event, intent }` |
| GET | `/onb/state` | — | Visitor's funnel position (resume) |
| GET | `/onb/products/{key}/motion` | — | Motion + CTA descriptor for one product. `404` unknown |
| GET | `/onb/entitlements` | — | Plan/entitlement snapshot for current subject |

### Sales leads
| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/onb/demo` | — | Submit demo/POC/contact request → `{ ok, requestId }`; fires emails |
| POST | `/onb/demo/{id}/schedule` | 🔒 `X-Internal-Secret` | Book a meeting for a request → `{ meeting_url, scheduled_at }` |

### Webhooks
| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/sms/zitadel` | 🔒 `ZITADEL-Signature` | Zitadel HTTP SMS provider → forwards OTP to SMS gateway |

### API docs (this reference)
| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/docs` | — | Swagger UI |
| GET | `/openapi.yaml` | — | OpenAPI 3 spec |

---

## Quick examples

```bash
# Health
curl http://localhost:8090/health

# Register (sets a session cookie into cookies.txt)
curl -c cookies.txt -X POST http://localhost:8090/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"firstName":"Ada","lastName":"Lovelace","email":"ada@acme.com","password":"correcthorse","consent":true}'

# Who am I (send the cookie back)
curl -b cookies.txt http://localhost:8090/auth/me

# Validate (the contract products/Kong use)
curl -b cookies.txt http://localhost:8090/validate

# Public product catalog
curl http://localhost:8090/auth/clients
```

> To exercise cookie-protected routes from **Swagger UI**, open `/docs` on the **same
> origin** as the API (the UI sends the session cookie automatically —
> `withCredentials` is on).
