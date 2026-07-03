# What Onboarding needs from Simplify Core

> **From:** Simplify Onboarding team · **Re:** driving the onboarding portal's product
> registry + plans from Core's public catalog.
> **Endpoint in use:** `GET https://api-dev.simplifyaipro.com/api/v1/public/catalog?app_id=simplify-hiring`

---

## Context (1 paragraph)

The Simplify onboarding portal ("one account, every product") shows the suite's **product
cards** on its home page and, going forward, will show **plans/pricing/entitlements**. We
want this driven by **Core** instead of hardcoded. We inspected the public catalog you gave
us (`app_id=simplify-hiring`, which you said is the master catalog holding all products).
It's great for **plans + pricing + features**, but a few things block using it as the
**product registry** for the home page. Details below.

---

## What we found in the master catalog (2026-07-03)

- Shape: `{ app_id, products[], addons[] }`. **6 products**, **4 addons**.
- Products present: `simplify-docflow`, `insights`, `talent`, `hiring-platform`,
  `simplify-hiring`, `Simplify_HR`.
- **Reliable fields:** `name`, `slug`, `currency`, `status`, `features[]`, `plans[]`
  (with `base_amount_cents`, `billing_period`, `trial_days`, `role`, and per-plan quotas).
- **Empty for (almost) all:** `tagline` (all), `website_url` (all), `logo_url` (only
  `Simplify_HR` set), `description` (empty for insights/talent), `highlight_features` (all),
  `banner_url` (all).

The onboarding home page needs **8** products; the catalog has **6**, and they don't line up.

---

## What we need — please action / confirm

### 1. Complete the product set (highest priority) 🔴
The home page suite is **8 products**; Core is missing **4** and has extras/dupes:

| Home page product | In Core? |
|---|---|
| DocFlow | ✅ `simplify-docflow` |
| Insights | ✅ `insights` |
| Talent | ✅ `talent` |
| Hiring | ⚠️ **two** entries — `hiring-platform` **and** `simplify-hiring` |
| **Legal** | ❌ missing |
| **Studio** | ❌ missing |
| **Transformer** | ❌ missing |
| **Credit** | ❌ missing |
| *(none)* | ➕ `Simplify_HR` exists but isn't a home card |

**Please:** add **Legal, Studio, Transformer, Credit**; merge the **two hiring** entries into
one; and tell us whether **SimplifyHR** should appear on the suite home page or stay hidden.

### 2. Populate the card fields per product (highest priority) 🔴
The home cards render from these, and they're empty:
- **`tagline`** — short one-liner under the product name.
- **`logo_url`** — product icon/logo (only SimplifyHR has one today).
- **`website_url`** — the product's **launch URL** (where "Open product" sends the user).

**Please:** populate these for every product — **or** confirm they should stay
**frontend-owned** (then we keep tagline/icon/launch-URL local and take only name + plans
from Core).

### 3. Canonical, consistent slugs + a mapping 🟠
Slugs don't match our product keys and are inconsistent (`simplify-docflow` vs `docflow`,
`Simplify_HR` casing, two hiring slugs).
**Please:** agree **one stable `slug` per product** and send the definitive
**slug → product** list so we can map reliably (and so slugs don't change under us).

### 4. Confirm the registry model 🟠
Today the whole suite is keyed under **`app_id=simplify-hiring`** — a *product-named* id.
That's fragile for a suite-wide registry.
**Please confirm one of:**
- (a) `app_id=simplify-hiring` is intentionally the permanent master catalog, **or**
- (b) there will be a neutral **`GET /api/v1/public/apps`** (or a neutral `app_id`) that
  returns the suite registry: `[{ app_id, name, tagline, logo_url, launch_url, status }]`.

  *(Option (b) is what we'd prefer — then onboarding reads the registry directly with nothing
  hardcoded.)*

### 5. Auth + production URL 🟡
We tested `https://api-dev.simplifyaipro.com/api/v1/public/catalog`, **unauthenticated**.
**Please confirm:**
- `/api/v1/public/*` stays **unauthenticated** (or give us the token/header if not).
- The **production base URL**.

---

## Ideal end state (for reference)

A single call returns the full suite with everything the portal needs:

```jsonc
GET /api/v1/public/apps            // (or the agreed master catalog)
[
  {
    "app_id": "simplify-docflow",
    "name": "SimplifyDocFlow",
    "tagline": "OCR, extraction and document workflows",
    "logo_url": "https://…/docflow.png",
    "launch_url": "https://docflow.simplifyai.id",
    "status": "PUBLISHED"
    // plans/features continue to come from the catalog per product
  },
  … all 8 products …
]
```

With #1 + #2 done, onboarding renders the home page **entirely from Core**. Until then we
run a safe fallback (local product list + overlay Core's names/plans), so nothing breaks —
but we can't show Legal/Studio/Transformer/Credit or the correct branding from Core yet.

---

*Questions or want a quick call to align on the slug list + the `/public/apps` shape? Happy to.*
