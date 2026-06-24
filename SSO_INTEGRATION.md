# Silent SSO — one login, no product login pages

**Goal:** sign in once on the Simplify onboarding portal; every product (DocFlow
first) is already logged in. No product shows its own login page. The session
lives until the user logs out.

## How it works (no DocFlow code change)

DocFlow's BFF already authenticates by reading the **`session_id` cookie** straight
from a **shared Redis** (`a.sessSvc.Get`) — it does not hardcode Zitadel. So we make
the onboarding service the **central session authority**:

1. **Same Zitadel** — the portal creates/validates users in the same Zitadel org DocFlow uses.
2. **DocFlow-schema session** — when the shared platform DB is wired (`DATABASE_URL`), the
   portal resolves the Zitadel sub → internal `app_users.id` + tenant + role and writes the
   session in **exactly DocFlow's schema** (`user_id`, `tenant_id`, `role`, `attrs.role_tier`,
   `custom_role_id`, `is_org_member`). Ported from `docflow-auth/internal/userstore`.
3. **Shared Redis** — the portal writes that session into **DocFlow's Redis** (`session:<id>`, DB1).
4. **Shared cookie** — the `session_id` cookie is host-only for `localhost` (shared across
   `:3100`/`:3000`); in production set `COOKIE_DOMAIN=.simplifyaipro.com`.
5. **Persist until logout** — `SESSION_PERSIST=true`: the session has no Redis expiry; only
   `/auth/logout` removes it.

Result: portal login → session in shared Redis → open DocFlow → its BFF reads the same
`session_id` from the same Redis → **logged in, no login page**. Logout on the portal
deletes the shared session → DocFlow is signed out too.

```
Portal sign-in ──► onboarding-service ──► writes session:<id> (DocFlow schema) ──► SHARED Redis (DB1)
                          │                                                            ▲
                          └── sets session_id cookie (localhost / .simplifyaipro.com)  │
Open DocFlow ──► docflow-bff reads session_id ───────────────────────────────────────┘  → user is in
```

## Run it locally

```bash
# 1) DocFlow stack up (creates the `redis`, `pgbouncer` services + network)
docker compose up -d                       # in the repo root

# 2) Onboarding ON DocFlow's network, sharing its Redis + DB
cd simplify-onboarding/simplify-onboarding-service
docker compose -f docker-compose.sso.yml --env-file ../../.env up -d --build
#   (if the network name differs: DOCFLOW_NETWORK=<name> docker compose -f … up -d)
#   find it with:  docker network ls | grep internal

# 3) Onboarding frontend
cd ../simplify-onboarding-web
npm run dev                                 # http://localhost:3100
```

**Test:** Portal (`:3100`) → Create account / Sign in → on Home click **Open SimplifyDocFlow**
→ DocFlow (`:3000`) opens **already logged in**. Sign out from the portal account menu →
reopen DocFlow → signed out.

## Verifying the wiring

- `GET http://localhost:8090/validate` (with the portal session cookie) → `{"valid":true,"user_id":"<internal-uuid>","tenant_id":"…","role":"…",…}`. `user_id` must be the **internal app_users UUID** (not the Zitadel numeric id) — that confirms `DATABASE_URL` is wired and the session is DocFlow-compatible.
- Onboarding startup log should show: `shared platform directory connected — DocFlow-compatible sessions enabled` and `session mode: persist-until-logout`.

## Production

- Put the portal at `id.simplifyaipro.com`, products at their subdomains, set `COOKIE_DOMAIN=.simplifyaipro.com` and `COOKIE_SECURE=true` so the one cookie is shared.
- Point each product's BFF session store at the shared Redis (DocFlow already is).

## Optional: never show a product login even on a cold open

If a user lands on DocFlow **without** a session, DocFlow currently redirects to its own
`/auth`. To send them to the portal instead, change DocFlow's "no session" redirect
(`docflow-frontend/src/services/apiClient.ts` / `App.tsx`) to
`http://localhost:3100/auth?returnTo=<docflow-url>`. Not needed for the common path
(sign in on the portal first).

## Other products (Drive, Legal, …)

Same recipe: each product that reads `session_id` from the shared Redis picks up the
central session automatically. Products that use Zitadel OIDC directly get SSO once the
shared session exists. Per-product RBAC beyond DocFlow's is added by extending the
session `attrs` (or moving to the OIDC bridge in the master plan, §P3).
