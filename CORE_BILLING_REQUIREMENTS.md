# Endpoints Onboarding needs from Core — subscription cards + purchase

> **From:** Simplify Onboarding team · **For:** SimplifyBilling Core team
> **Goal:** the portal home cards should show, **per product, the signed-in user's plan**
> (or a **Purchase** button if not subscribed), with purchase redirecting to Core.
> **Verified against:** `https://api-dev.simplifyaipro.com` with the master `bk_` key
> (`app_id=simplify-core`), 2026-07-08.

---

## TL;DR — we need **2 new endpoints**; everything else already works

| # | Endpoint | Status | Purpose |
|---|---|---|---|
| **1** | `GET /api/v1/sdk/subscriptions/{org_id}` | ❌ **404 — please build** (already on your `PLAN.md` "To ADD") | per-**product** subscription/plan for a user → fills the cards |
| **2** | `POST /api/v1/sdk/checkout` (or a checkout **URL pattern**) | ❌ **missing** | purchase redirect to Core for a product+plan |

Everything below in "Already works" we've tested and will reuse as-is.

---

## Already works (baseline — no change needed) ✅

- **One master key sees all products.** `GET /api/v1/public/catalog?app_id=simplify-core`
  returns all 7 products, each with plans (`base_amount_cents`, `billing_period`, `trial_days`).
  We already drive the cards' names/taglines/logos/plans from this.
- **user = org, resolvable idempotently.** `POST /api/v1/sdk/provisioning/orgs`
  `{external_id, name, owner_user_id, owner_email}` → `{org_id, created}`. Called twice with the
  same `external_id` returns the **same** `org_id` (`created:false`). We pass the user's **Zitadel
  `sub`** as `external_id`, so we can resolve `user → org_id` on every login.
- **Quota/entitlement snapshot exists.** `GET /api/v1/sdk/entitlements/{org_id}` → merged
  features + remaining/quota (fine for quota display; **not** per-product plan — see gap #1).

---

## Endpoint 1 — per-product subscription list  🔴 (the blocker)

**Why:** `entitlements/{org}` returns a **single merged** snapshot (`app_id:"simplify-core"`, one
ambiguous `plan_code`, features merged with **no product tag**). With an org subscribed to
`docflow-pro` **and** insights `Pro-Monthly`, it showed just `plan_code:"Pro-Monthly"` — we
**cannot** derive per-product plans from it. `GET /sdk/subscriptions/{org_id}` currently **404s**.

```
GET /api/v1/sdk/subscriptions/{org_id}
Authorization: Bearer <bk_ master key or portal credential>
```

**Response 200** — one row **per product** (keyed by the **product slug**, matching the catalog):
```jsonc
{
  "org_id": "4c775a84-…",
  "subscriptions": [
    {
      "product": "simplify-docflow",          // product slug — MUST match catalog "slug" (NOT the master app_id)
      "plan_code": "docflow-pro",
      "plan_name": "Docflow Pro",
      "state": "ACTIVE",                        // ACTIVE | TRIALING | PAST_DUE | CANCELED | EXPIRED
      "is_trial": false,
      "current_period": { "start": "2026-07-08T…Z", "end": "2026-08-08T…Z" },
      "cancel_at_period_end": false
    },
    {
      "product": "insights",
      "plan_code": "Pro-Monthly", "plan_name": "Pro-Plan",
      "state": "ACTIVE", "is_trial": true,
      "current_period": { "start": "…", "end": "…" }, "cancel_at_period_end": false
    }
    // …one row per product the org has a subscription for
  ]
}
```

**Rules that make it work for us**
- **One call, all products** for the org (don't make us loop per product).
- **`product` must be the catalog slug** (`simplify-docflow`, `insights`, …) so we can match a
  subscription to its card. The merged `app_id:"simplify-core"` is useless for this.
- **Products not in the array = not subscribed** → the card shows **Purchase**. (Or include them
  with `"state":"NONE"` — either is fine; just be consistent.)
- **Free plans:** if a "free" plan counts as a subscription, include it (`plan_code:"docflow-free"`,
  `state:"ACTIVE"`); we'll show "Free".
- *(Optional, nice-to-have)* include this product's **quota summary** inline
  (`"features":[{code, remaining, total}]`) so we don't also call `entitlements`.

**Status codes:** `200` (even with empty `subscriptions`), `401` bad key, `404` unknown org.

---

## Endpoint 2 — purchase / checkout redirect  🔴

**Why:** we redirect the user to Core to buy. There's no "buy this plan → URL" today
(`payment_options` is freeform + only appears on quota-exhaustion; `/sdk/addons/purchase` is
noted as a bug). Either shape works for us:

**Option A — hosted checkout session (preferred):**
```
POST /api/v1/sdk/checkout
Authorization: Bearer <bk_ master key or portal credential>
{
  "org_id": "4c775a84-…",
  "product": "simplify-docflow",
  "plan_code": "docflow-pro",
  "return_url": "https://onboarding.simplifyai.id/?checkout=success&product=simplify-docflow",
  "cancel_url": "https://onboarding.simplifyai.id/?checkout=cancel"
}
→ 200 { "checkout_url": "https://core.simplifyaipro.com/checkout/sess_…" }
```
We redirect the browser to `checkout_url`; Core sends the user back to `return_url` after.

**Option B — documented URL pattern** (if checkout is a plain page):
`https://core.simplifyaipro.com/checkout?org=<org_id>&product=<slug>&plan=<plan_code>&return=<url>`
— just publish the exact pattern + required params.

*(Optional)* **Manage/billing-portal URL** for already-subscribed products:
`POST /api/v1/sdk/billing-portal { org_id, return_url } → { url }` — so the card's **Manage**
button opens Core's billing portal.

---

## Auth & identifiers — please confirm
1. **Which credential** should the onboarding portal use for #1/#2 — the existing **master `bk_`
   key** (`app_id=simplify-core`, which already reads all products), or a **dedicated read-only
   portal credential**? (We're fine with either; the master key already works.)
2. **Product identifier consistency:** the public catalog uses `slug` (`simplify-docflow`), the
   entitlements/subscription side uses `app_id`. Endpoint #1's `product` field **must** be the
   catalog `slug` so we can join to the card. Please confirm they're the same string.
3. **Prod base URL** for these (we currently use `api-dev.simplifyaipro.com`).

---

## How onboarding will consume it
1. On login, resolve `org_id` = `provisioning/orgs(external_id = zitadel_sub)`.
2. Call **#1** → attach `{product → plan_code, plan_name, state}` to the product list we already
   serve at `/auth/clients` (which is built from the public catalog).
3. Cards render **plan name** (e.g. "Docflow Pro") when subscribed; **Purchase** button otherwise
   → **#2** (redirect to Core), returning to the portal after.

**Blocking: #1 (per-product subscription list) + #2 (checkout).** With those two, we ship the
subscription cards and purchase flow. All other pieces are verified working.
