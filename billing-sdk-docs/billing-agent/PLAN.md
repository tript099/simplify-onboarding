# Billing Agent — product-side container/SDK

**Goal:** one batteries-included Go artifact every product copies/runs so they never
re-implement quota gating, entitlement checks, usage metering, subscription/credit
reads, or provisioning against SimplifyBilling.

Ships in **two modes** off the same code:
1. **Sidecar container** (`billing-agent`) — runs next to any product (any language).
   The product calls it on `http://localhost:8099` with a tiny unauthenticated local
   API; the agent holds the API key, caches, batches, retries, and talks to core.
2. **Go library** (`libs/billing-sdk-go`) — Go products import directly, no sidecar.

The agent is a thin orchestration layer over the EXISTING `/api/v1/sdk/*` surface +
the existing SDK packages (`entitlement`, `metering`, `provisioning`, `webhook`).

---

## Why a sidecar (not just a library)
- **Language-agnostic**: Python/Node/Java products call local HTTP, no Go required.
- **One place** for the API key (never in product code), cache, usage batching,
  circuit-breaker, and fail-open/closed policy.
- **Resilience**: serve stale entitlements when core is briefly down; buffer usage and
  flush at-least-once with idempotency keys.

---

## Mental model: a Billing PDP — the Cerbos of quota
Cerbos answers *"can this principal do this action?"* → ALLOW/DENY. The agent answers
the **billing** question with the same ergonomics:

> *"Can org X use service Y — and if so, how much?"* … then *"org X just used N."*

The product/BFF makes **two dead-simple calls** and the agent hides ALL billing logic
(entitlement resolution across plan + add-ons + token pools, overage, daily/period
windows, wallet/Simplify-Credit funding, caching, batching, reporting upstream):

```
1) CanUse(org, service)        → { allow, remaining, limit, reset_at, reason }
2) Report(org, service, units) → { ok, remaining }        // after the work is done
```

For strict limits where a check-then-report race would overspend, use the **atomic**
variant (single call that decrements only if allowed):

```
Authorize(org, service, units) → { allowed, remaining, funding_source, payment_options }
```

## What the product calls — Agent local API (`localhost:8099`)
Unauthenticated (localhost-only); the agent injects the product API key upstream.

| Method | Path | Maps to | Purpose |
|---|---|---|---|
| POST | `/v1/can-use` | entitlement check (read) | **"Can org use service + how much?"** → `{allow, remaining, limit, reset_at, reason}`. No decrement. The Cerbos-style gate. |
| POST | `/v1/report` | metering consume | **"Org used N."** Decrements quota after the work — `{ok, remaining}`. Batched + idempotent. |
| POST | `/v1/authorize` | metering consume (atomic) | Check **and** decrement in one call (no race) → `{allowed, remaining, funding_source, payment_options}`. Use for strict/expensive features. |
| GET | `/v1/features?org_id=` | entitlements snapshot | All features + quota/remaining + plan (dashboards). |
| GET | `/v1/subscription?org_id=` | subscription detail | Plan, state, period, trial, active add-ons. |
| GET | `/v1/credit?org_id=` | credit balance | Simplify Credit balance + branding. |
| POST | `/v1/provision/org` · `/v1/provision/trial` | provisioning | Onboard org / start trial (idempotent). |
| GET/POST | `/v1/addons` · `/v1/addons/buy` | addons | List/buy top-ups. **(roadmap — not implemented in the agent yet)** |
| GET | `/healthz` · `/readyz` | — | Liveness/readiness. |

**Hot-path contract (mirrors Cerbos `CheckResource`):** the BFF calls `/v1/can-use`
before serving; on `allow:false` it returns 402/quota-exceeded with `reason` +
`payment_options`; after the work, `/v1/report` (or just use `/v1/authorize` to fuse both).

---

## Platform endpoints the agent calls (`/api/v1/sdk/*`, `Authorization: Bearer <bk_… or Zitadel JWT>`)

### Already exist ✅
- `GET  /api/v1/sdk/entitlements/{org_id}` — full entitlement snapshot.
- `POST /api/v1/sdk/entitlements/{org_id}/invalidate` — bust cache.
- `POST /api/v1/sdk/metering/consume` — consume N units; `402` + payment options when over quota (this IS the gate).
- `POST /api/v1/sdk/metering/consume/batch` — batched consume (at-least-once).
- `POST /api/v1/sdk/provisioning/orgs` — ensure org.
- `POST /api/v1/sdk/provisioning/subscriptions/trial` — start trial.
- `GET  /api/v1/sdk/catalog/export` — products/plans/features catalog.

### To ADD (gaps for a complete product kit) ⬜
- `GET  /api/v1/sdk/subscriptions/{org_id}` — subscription detail (state, plan, period, trial, active add-ons). *Today products can only infer plan from entitlements.*
- `GET  /api/v1/sdk/credit/{org_id}` — Simplify Credit balance + resolved config (name/symbol/logo/rate). *New credit feature has no SDK surface yet.*
- `GET  /api/v1/sdk/addons` + `POST /api/v1/sdk/addons/purchase` — add-on list/buy under API-key auth. *Current SDK addon methods call the session-auth `/client` routes and won't work with a product API key — real bug.*
- `POST /api/v1/sdk/entitlements/{org_id}/check` — batch-check several features in one call (dashboard/preflight) to cut round-trips.
- (optional) `GET /api/v1/sdk/feature-config/{org_id}` — per-feature provisioning flags/payload (custom role payloads per product) — aligns with "user-provisioning setup is product-level".

All new routes mount under the existing `r.Route("/sdk", RequireProductAuth)` group (product
API key **or** Zitadel JWT) and reuse the same service methods as their `/client` or `/admin`
counterparts.

---

## Resilience & semantics (built into the agent)
- **Gate atomicity**: `/v1/authorize` maps to `metering/consume` — check + decrement is one
  atomic server op (no TOCTOU). `/v1/can-use` is read-only (may be slightly stale).
- **Entitlement cache**: in-proc + optional Redis, TTL ~30–60s, **stale-on-error**
  (serve last-known if core is down). Reuses `entitlement` pkg cache + `GetStale`.
- **Fail policy per feature**: `fail_open` (allow if billing unreachable, for non-critical)
  vs `fail_closed` (deny). Configurable; default closed for paid features.
- **Usage batching**: buffer + flush every N events / T seconds via `metering.BatchRecorder`;
  idempotency keys → at-least-once, dedup server-side.
- **Cache warming via webhooks**: optional `/webhooks` receiver verifies HMAC and on
  `subscription.created/updated/canceled` invalidates/warms the org's entitlement cache.
- **Retries + circuit breaker** with backoff; structured logs; Prometheus `/metrics`.

---

## Config (env)
```
BILLING_BASE_URL     = https://core.simplify…         # platform
BILLING_API_KEY      = bk_…                            # per-app, rotated in admin → Integration
AGENT_LISTEN         = :8099
AGENT_CACHE_TTL      = 45s
AGENT_FLUSH_INTERVAL = 2s                              # report batch flush interval (0 disables)
AGENT_FLUSH_SIZE     = 50                              # report batch flush size (0 disables)
AGENT_FAIL_MODE      = closed                          # or open
# Zitadel M2M (instead of BILLING_API_KEY):
ZITADEL_TOKEN_URL / ZITADEL_CLIENT_ID / ZITADEL_CLIENT_SECRET / ZITADEL_SCOPE
```
API keys are minted/rotated today via the admin Integration endpoints (`bk_`-prefixed,
hashed at rest).

---

## Phases
- **P1 — Platform gaps**: add `/sdk/subscriptions/{org}`, `/sdk/credit/{org}`,
  `/sdk/addons(+purchase)`, batch entitlement check. (Thin wrappers over existing services.)
- **P2 — Agent core**: Go service wrapping the SDK packages with the local API above
  (`gate`, `check`, `features`, `subscription`, `credit`, `usage`, `provision`, `addons`).
- **P3 — Resilience**: cache + stale-on-error, batching, fail modes, circuit breaker, metrics.
- **P4 — Container**: distroless image, non-root, read-only FS; `docker-compose` snippet +
  k8s sidecar example; `healthz`/`readyz`.
- **P5 — DX**: `copy-me` quickstart, a 20-line example product, OpenAPI for the local API,
  versioned release of `libs/billing-sdk-go`.

## Out of scope (platform already owns)
Billing/invoicing, payment collection, dunning, the portals — products only *read*
entitlements/subscription/credit and *report* usage; they never touch money directly.

---

## Status — BUILT + E2E (2026-06-22)
Shipped in `/billing-agent` (own Go module, `.env`, distroless Dockerfile) and wired into
docker-compose as the `billing-agent` service (`:8099`).

- Endpoints live: `/v1/can-use`, `/v1/report`, `/v1/authorize`, `/v1/features`,
  `/v1/subscription`, `/v1/credit`, `/v1/provision/{org,trial}`, `/healthz`, `/readyz`.
- Maps to existing `/api/v1/sdk/entitlements` + `/sdk/metering/consume`; subscription +
  credit derived from the entitlement snapshot (no new core endpoints required).
- Resilience: entitlement cache (TTL) + **stale-on-error**, per-deploy **fail-open/closed**,
  cache invalidation on consume, idempotency keys auto-minted.
- Platform fix needed: added the missing `products.api_key_hash` column (migration drift) so
  product API-key auth resolves (it 500'd before).

**e2e (live, 5/5)**: `can-use` allow + remaining=16; `authorize` 1 unit → remaining 15
(funding SUBSCRIPTION_QUOTA); `report` 2 units; `can-use` → remaining 13 (real platform
decrement); `subscription` → piroo/TRIALING; `credit` → 97,640 SBC; unknown feature → deny.

---

## Auth: API key vs Zitadel (M2M)
**Module layout** (`billing-agent/internal/`): `config` · `auth` (pluggable Authenticator)
· `upstream` (core /sdk client + types) · `pdp` (cache + can-use/report/authorize + HTTP).
`main.go` is wiring only.

**Yes, Zitadel can replace API keys** — the agent already supports it as a config switch:
each tenant gets a **Zitadel service user (machine user)**; set `ZITADEL_TOKEN_URL` +
`ZITADEL_CLIENT_ID/SECRET/SCOPE` and the agent fetches short-lived tokens via
**client-credentials**, caches them (until ~60s before expiry), and presents them as the
Bearer to core — instead of a static `bk_` key.

| | Static API key (today) | Zitadel M2M |
|---|---|---|
| Secret lifetime | long-lived `bk_` | short-lived JWT (auto-refreshed) |
| Rotation / revocation | manual (admin → Integration) | central, instant in Zitadel |
| Scopes / roles / audit | none | project roles + Zitadel audit |
| Tenant (app_id) mapping | `products.api_key_hash` → app_id | a JWT claim (project role / metadata) → app_id |
| Extra moving parts | none | token endpoint + claim mapping |

**The one missing platform piece:** core's `/sdk` group only knows API keys today
(`RequireProductAPIKey`); Zitadel JWTs are validated at the *edge* (auth-service/BFF), not
in core. To accept agent JWTs directly, add a sibling middleware **`RequireProductJWT`**:
verify the token against `ZITADEL_JWKS_URL` (iss/aud/exp), read `app_id` from a claim, set
`X-App-ID` — then the `/sdk` handlers are unchanged. Mount it as: try JWT, fall back to API
key (hybrid), so both work during migration.

**Recommendation:** keep **both**. API keys for quick/simple product integrations; Zitadel
M2M for enterprise tenants that want central identity, rotation, and audit. The agent is
already auth-agnostic; only `RequireProductJWT` on core remains to make the Zitadel path live.

---

## Zitadel M2M — IMPLEMENTED + e2e (2026-06-23)
The "missing piece" (`RequireProductJWT` on core) is built, so the agent's Zitadel mode
now works end-to-end.

- **Core**: `httpx.JWTVerifier` (JWKS RS256 + iss/exp via go-jose v4) + `RequireProductAuth`
  — a **hybrid** /sdk middleware: `Bearer bk_…` → API key; any other bearer → Zitadel JWT,
  verified and mapped `sub → app_id` via the new `apps.zitadel_subject` column
  (`integration.Repo.ResolveSubject`). API-key callers are unaffected.
- **Config**: `ZITADEL_ISSUER` (+ existing `ZITADEL_JWKS_URL`); JWT path enabled only when both set.
- **Agent**: already had the client-credentials authenticator (`internal/auth.zitadel`) —
  set `ZITADEL_TOKEN_URL` / `ZITADEL_CLIENT_ID` / `ZITADEL_CLIENT_SECRET` / `ZITADEL_SCOPE`
  and it fetches/caches short-lived tokens instead of a static key.

**Mapping a tenant**: set `apps.zitadel_subject` = the Zitadel machine-user id (token `sub`).

**e2e (live, real Zitadel @ auth.simplifyaipro.com)**: created a dedicated machine user
`billing-agent-test` (JWT access tokens) → client_credentials token → agent (no API key,
`auth=zitadel_m2m`) → core verified JWT, mapped sub→simplify-hiring → `can-use` allow,
`authorize` consumed (remaining 15), `subscription` read. API-key path still 200; bogus JWT → 401. 5/5.
