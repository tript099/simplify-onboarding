# Session Architecture — Redis load & Zitadel

How authenticated sessions flow from the onboarding platform to every product, whether
the shared Redis becomes a bottleneck, and whether products can read session/login data
directly from Zitadel instead.

> Code references: [`internal/session/session.go`](simplify-onboarding-service/internal/session/session.go),
> [`internal/handler/auth.go`](simplify-onboarding-service/internal/handler/auth.go) (`Validate`, `establishSession`).

---

## How it works today

- The browser holds only an **opaque `session_id` cookie** — HttpOnly, `Secure`, scoped
  to `.simplifyai.id` so it is sent to every product subdomain. No identity or token
  material is ever exposed to JavaScript (deliberate XSS-safety posture).
- The real identity lives **server-side in the onboarding Redis** as
  `session:<uuid> → JSON` (Zitadel `sub`, email, name, verified flags, role, …).
- Products resolve a request in one of two ways:
  1. **HTTP `/validate`** — call onboarding `GET /validate` with the cookie; it returns
     the universal identity `{ valid, sub, email, … }`. Cost: **one Redis `GET` + one
     TTL touch**.
  2. **Direct shared-Redis read** — DocFlow's BFF reads the same `session:<id>` key
     directly. The session JSON schema was intentionally matched to `docflow-auth`, so
     no translation is needed.
- Each product maps the universal `sub` to its **own** internal user id / tenant / role
  in its **own** database. The onboarding service asserts nothing product-specific.

```
Browser ──cookie(session_id)──▶ Product/Gateway ──/validate (or direct Redis)──▶ Onboarding Redis
                                        │
                                        └─ maps sub ─▶ product's own DB (user, tenant, role)
```

---

## Will the shared Redis be overloaded?

**Redis itself: almost certainly not, for a long time.**

- A validate is a single **O(1) `GET`** of a ~1 KB value plus a TTL touch. A modest
  single Redis node handles **100k–1M simple ops/sec**.
- Memory is trivial: **1M live sessions × ~1 KB ≈ 1 GB**.
- Sessions shard perfectly by key, so **Redis Cluster** scales horizontally the day it's
  ever needed.

**The real pressure points are not Redis — they are:**

1. **The `/validate` service becomes a shared hot path.** If every product calls
   `/validate` on *every* request with no caching, the onboarding Go service absorbs the
   **union** of all products' traffic, and every request pays a **network hop** of
   latency. This is the bottleneck long before Redis is.
2. **Every read is also a write.** `Get()` re-arms the sliding TTL on every read
   (`EXPIRE`), and in persist mode calls `PERSIST` on every read
   ([`session.go:115-123`](simplify-onboarding-service/internal/session/session.go#L115-L123)).
   So validates can't be served from read replicas — everything hits the primary.
3. **Blast radius / coupling.** If the onboarding Redis or the validate service is down,
   **every product loses auth at once.**

### Mitigations (no redesign required)

- **Cache the validation result** at the gateway (Kong) or in each product, keyed by
  `session_id`, for ~30–60s. Collapses N requests → 1 validate per window. *Biggest lever.*
- **Validate once at the edge** (Kong) per request, not in each microservice.
- **Redis HA + Cluster** (Sentinel / ElastiCache) for failover and sharding.
- **Trim TTL writes:** only `PERSIST` when a TTL actually exists; only re-arm the sliding
  TTL when it has dropped below ~half. Cuts write volume on the hot path.

---

## Can products read session data directly from Zitadel?

Two very different things hide behind this question:

### A) Introspect / query Zitadel per request — ❌ worse

Zitadel is a **Postgres-backed IdP**, not a session cache. A Zitadel token-introspection
call is **far heavier** than a Redis `GET`. Pointing every product request at Zitadel
would overload Zitadel *faster* than Redis would ever overload. **Don't do this.**

### B) Stateless JWT validation via JWKS — ✅ the real scale answer

Let Zitadel issue **signed JWT access/ID tokens**. Each product verifies the token
**locally** against Zitadel's public keys (JWKS, cached for hours) — **zero central
round-trips**: no onboarding call, no Redis, no Zitadel call on the hot path. The `sub`
is a claim inside the token.

The trade-offs are exactly why the current design uses opaque sessions instead:

| | Opaque session + Redis `/validate` (today) | Stateless JWT from Zitadel |
|---|---|---|
| Per-request cost | Central lookup (Redis GET / HTTP hop) | Local signature check, no round-trip |
| Token in browser | **No** — stays server-side (XSS-safe) | Yes — BFF or client holds a bearer token |
| Instant logout / revocation | **Yes** — delete the key | Hard — valid until expiry (needs short TTL + refresh, or a revocation list) |
| Blast radius on validate | Shared Redis / service | None |

---

## Recommendation

You don't have to pick one — the scalable, low-risk shape is a **hybrid**, most of which
already exists here:

1. **Keep opaque-session + BFF** for browser-facing auth — it gives the two things JWTs
   make painful: **no tokens in the browser** and **instant revocation**.
2. **Add short-lived caching of `/validate`** at Kong so Redis and the validate service
   see a fraction of request volume. This alone removes the "overload" worry for a long
   time.
3. **Use JWKS-verified JWTs for service-to-service / API** calls where per-request
   latency matters more and instant revocation matters less.
4. Treat "read directly from Zitadel per request" as a **non-option** — Zitadel is the
   issuer, not the per-request session store.

**Bottom line:** Redis is *not* the near-term risk — an **uncached shared validate path**
is. Fix that with edge caching before reaching for a token rewrite.
