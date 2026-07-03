# Simplify Core — Catalog Integration Plan

> Goal: stop hardcoding the product cards on the onboarding home page and drive them
> (and later plan/pricing/entitlement data) from **Simplify Core**.
>
> Status: **✅ Phase 1 BUILT (2026-07-03).** The onboarding catalog now overlays Core's
> master catalog (`app_id=simplify-hiring`): names, taglines, launch URLs and logos come
> from Core where present, Core-only products are added, and products Core lacks stay as
> built-ins (fallback-safe, refreshed every 5 min). Config: `SIMPLIFYCORE_BASE_URL` +
> `SIMPLIFYCORE_CATALOG_APP_ID`. Code: `internal/coreclient` + `internal/catalog` overlay,
> frontend `products.ts`/`ProblemCard.tsx`. §6 (Core-side gaps) still pending.

---

## 1. TL;DR

- **One endpoint, one call** gets the whole catalog:
  `GET https://api-dev.simplifyaipro.com/api/v1/public/catalog?app_id=simplify-hiring`
  (`app_id` required — as query param or `X-App-ID` header; no auth on this `public` route).
- It returns **`{ app_id, products[], addons[] }`** — a **billing catalog**: each product has
  **features** (capability definitions), **plans** (pricing tiers + per-plan quotas), plus
  suite-wide **addons**.
- It currently holds **6 products**, but the onboarding home page shows **8**. They don't
  line up (see §4) — and the fields the cards need (**tagline, logo, launch URL**) are mostly
  **empty**. So Core is a great source for **product names + plans/pricing**, but **not yet a
  complete drop-in** for the home-page registry. §5 = how we ship anyway; §6 = what we need
  from Core to make it clean.

---

## 2. The endpoint

```
GET https://api-dev.simplifyaipro.com/api/v1/public/catalog?app_id=simplify-hiring
# app_id can also be a header:  X-App-ID: simplify-hiring
```
- `app_id` **required** → else `400 {"error":{"code":"app_id.required", …}}`.
- **No auth** on `/public/*` (a `SIMPLIFYCORE_TOKEN` config slot already exists if that changes).
- Base URL tested is **`api-dev`** — need the **prod** base URL.

---

## 3. What each product carries (live inventory, 2026-07-03)

`products[]` — **6 products**:

| slug | name | description | logo_url | website_url | plans (base price/mo) | #features |
|---|---|---|---|---|---|---|
| `hiring-platform` | SimplifyHiring Platform | "Unified plan catalog for … client and vendor orgs" | — | — | 7 · client/vendor/candidate roles · $0–$380 | 20 |
| `simplify-docflow` | SimplifyDocflow | "AI-powered document management" | — | — | 3 · Free / Pro **$10** / Pro Plus | 40 |
| `insights` | Insights | *(empty)* | — | — | 2 · Free / Pro **$25** | 33 |
| `Simplify_HR` | SimplifyHR | "360 degree HR functions through AI" | **✅ set** | — | 1 · HR Pro **$10** (15-day trial) | 3 |
| `talent` | SimplifyTalent | *(empty)* | — | — | 2 · Free (weekly) / Pro **$50** | 33 |
| `simplify-hiring` | Simplify Hiring | "AI based Hiring Platform" | — | — | 2 · Candidate Pro / Best Pro | 10 |

**Per-product fields:** `id, slug, name, description, tagline, logo_url, banner_url,
website_url, highlight_features[], currency, tax_category, status, features[], plans[]`.

**`features[]`** (capability catalog): `{ id, code, name, description, type(BOOLEAN|METERED),
unit, aggregation }`.

**`plans[]`** (pricing tiers): `{ id, code, name, description, billing_period(MONTHLY|WEEKLY),
trial_days, base_amount_cents, currency, status, credit_grant_cents, role, role_locked,
features[], group_limits }` — where the plan's **`features[]`** are the actual entitlements:
`{ feature_code, type, unit, included_quota, overage_allowed, overage_cents }`.

**`addons[]`** — 4, e.g.
`{ code:"ai_tok_addon", name:"tok addon", sku_type:"token_pack", price_cents:100,
currency:"USD", is_one_time:true }`.

### What's populated vs empty (important)
- ✅ Reliable: `name`, `slug`, `currency`, `status`, `features`, `plans` (incl.
  `base_amount_cents` pricing, `billing_period`, `trial_days`, `role`).
- ⚠️ Empty for almost all: **`tagline`** (all), **`website_url`** (all — this is the "launch"
  link), **`logo_url`** (only `Simplify_HR` has one), `description` (empty for insights, talent),
  `highlight_features` (all), `banner_url` (all).
- ❌ Not in Core at all: **icon/accent** (pure frontend), a **card "intent"** string.

---

## 4. Gap: Core catalog vs the home page (⚠️ the thing to fix)

The onboarding home shows **8** products; Core has **6**, and they don't map 1:1:

| Home page (8) | In Core? |
|---|---|
| docflow | ✅ `simplify-docflow` |
| insights | ✅ `insights` |
| talent | ✅ `talent` |
| hiring | ⚠️ **two** entries: `hiring-platform` **and** `simplify-hiring` |
| **legal** | ❌ missing |
| **studio** | ❌ missing |
| **transformer** | ❌ missing |
| **credit** | ❌ missing |
| *(none)* | ➕ `Simplify_HR` exists in Core but is **not** a home card |

So today:
- **4 of the 8 home products are absent** from Core (legal, studio, transformer, credit).
- Core has an **extra** product (SimplifyHR) and a **duplicate/ambiguous** hiring entry.
- **Slugs don't match** the home keys (`simplify-docflow` vs `docflow`, `Simplify_HR` casing…).
- **Presentation fields are empty**, so cards can't be rendered from Core alone.

⇒ We **cannot** render the home page from Core as-is; we'd lose 4 cards and all the styling.

---

## 5. Plan (ship-safe, phased)

**Phase 1 — Core enriches, local stays the base (fallback-safe).**
- Onboarding backend keeps its **hardcoded 8-product list as the base/fallback**, and
  **overlays Core data by slug** where present (name, description, and — Phase 2 — plans).
  A card is never dropped because Core lacks it.
- **Presentation stays local** (`icon`, `accent`, `launchUrl`) in the frontend `products.ts`;
  `hydrateProduct()` already merges backend data with local icons, so minimal FE change.
- Add a **slug map** (Core `simplify-docflow` → home `docflow`, etc.).
- Fetch once, **cache ~5 min**, timeout, **fall back to hardcoded** on error/empty. Keep the
  existing `SIMPLIFYCORE_STUB=true` as the "ignore Core" switch.

**Phase 2 — surface the real value: plans + pricing + entitlements.**
- Core's genuine advantage over our hardcode is **plans/pricing/quotas**. Use them where the
  portal shows trial scope / "what you get" (today `TrialScope` is hardcoded in `catalog.go`),
  and feed the existing **`entitlements`** client from the same Core data.

**Phase 3 — registry-driven (once Core is complete).**
- When Core holds **all 8 products with populated marketing fields + matching slugs**, drop
  the hardcoded base and read the suite straight from the master catalog. Icon/accent stay local.

---

## 6. What we need from the Core team

1. **All 8 suite products in the master catalog** — add **legal, studio, transformer, credit**;
   resolve the **two hiring entries** (`hiring-platform` vs `simplify-hiring`) into one, and
   confirm whether **SimplifyHR** should be a home card or stay hidden.
2. **Populate the card fields** per product: **`tagline`**, **`logo_url`**, **`website_url`**
   (= the product's launch URL) — or tell us these stay frontend-owned (then we keep icon +
   tagline + launchUrl local and only take name/plans from Core).
3. **Canonical, stable, consistent `slug`s** that map to the suite (agree one id per product;
   fix casing like `Simplify_HR`). Give us the **exact slug→product mapping**.
4. **Confirm the registry model:** is `app_id=simplify-hiring` intended to be THE master
   catalog for the whole suite long-term, or should there be a dedicated
   `GET /api/v1/public/apps` (or a neutral `app_id`)? We don't want the suite registry keyed
   under a product-named id.
5. **Auth + prod:** confirm `/public/catalog` stays **unauthenticated**, and give the
   **production base URL** (we've only tested `api-dev`).

**Highest priority:** #1 + #2 (complete product set + card fields) — that's what unblocks
rendering the home page from Core. Until then we ship Phase 1 (local base + Core overlay).

---

## 7. Where the code lands (Phase 1)

| File | Change |
|---|---|
| `internal/config/config.go` | reuse `SimplifyCoreURL/Token/Stub`; add master `app_id` + slug map |
| `internal/coreclient/*.go` (new) | catalog HTTP client + cache + timeout + fallback |
| `internal/catalog/catalog.go` | hardcoded base, overlay Core by slug (never drop a card) |
| `simplify-onboarding-web/src/lib/products.ts` | unchanged data; keep icon/accent/launchUrl overlay |
| `.env` / `.env.example` | `SIMPLIFYCORE_BASE_URL`, master `app_id`, keep `SIMPLIFYCORE_STUB` |

*This document is the plan; nothing is implemented yet. Confirm §6 (esp. #1–#2), then I build Phase 1.*
