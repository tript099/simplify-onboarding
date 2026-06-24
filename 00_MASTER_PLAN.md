# Simplify Onboarding — Master Plan

**Initiative:** One account, every Simplify product — a value-first onboarding + single
sign-on system across all 8 products.
**Status:** Planning
**Source of truth for CX:** `Simplify_Onboarding_CX.pdf` (value-first customer experience design)
**Companion build plans:**
[Backend](SIMPLIFY_ONBOARDING_BACKEND_PLAN.md) · [Frontend](SIMPLIFY_ONBOARDING_FRONTEND_PLAN.md)

---

## 0. What changed (why this doc exists)

The first cut planned a narrow SSO auth module. The CX design widens it into a
**value-first onboarding system**. The single biggest lever on drop-off is **ordering the
journey by commitment, not by the data we want to collect**:

```
See value → small commitment → more value → register → verify → pay / convert
```

Every screen we build must ask for the *minimum needed to deliver the next bit of value*.
Registration and verification move **later**; identity *capture* stays early. This master
plan is the anchor; the backend and frontend plans implement it.

---

## 1. Scope — what this module builds vs. integrates with

| We BUILD (this module) | We INTEGRATE with (exists / owned elsewhere) |
|---|---|
| Branded **onboarding web** (homepage problem-router, product landing pages, try-before-register, registration wall, OTP verify, sign-in) | **Zitadel** — authentication / identity (existing IdP, one global org) |
| **simplify-onboarding-service** (Go) — anonymous session + intent capture, registration, email+mobile OTP, multi-product SSO bridge, product/entitlement reads | **Cerbos** — authorization (existing `cerbos/`, [CERBOS_RBAC_SYSTEM_PLAN.md](../CERBOS_RBAC_SYSTEM_PLAN.md)) |
| Product registry + per-product motion config | **SimplifyCore** — provisioning, subscriptions, entitlements, billing (platform layer; we *read* entitlements, never store billing) |
| Activation hooks (first-run seed triggers, checklist state, behavioral-email triggers) | **MailForge** — email send ([project] centralised email) |
| CTA framework (commitment ladder, three-zone embed) | **Product apps** (DocFlow, Legal, Drive, …) — own their workspaces; we deep-link + hand off |
| Analytics + company-identification wiring | **PostHog/analytics**, **CRM**, **reverse-IP** (Clearbit/RB2B/Leadfeeder), **SMS provider** |

> **Boundary rule:** we never re-implement billing (SimplifyCore owns it) or per-resource
> authz (Cerbos owns it). The onboarding module gets the right user to the right product,
> captures identity at the right moment, and establishes the SSO session.

---

## 2. The eight products & their motions (from CX doc)

| Product | Allowed user types | Asks user type? | Self-serve activation event |
|---|---|---|---|
| SimplifyStudio | Enterprise, Self-serve | Yes (2-way) | Created one use case + generated a PRD |
| SimplifyTransformer | Enterprise only | No | Saw a modernization assessment on a sample snippet |
| SimplifyCredit | Enterprise, Self-serve | Yes (2-way) | Ran credit analysis on self/1 company |
| SimplifyLegal | Enterprise, Self-serve | Yes (2-way) | Got a usable AI Lawyer / doc-review result |
| SimplifyDocFlow | Enterprise, Self-serve | Yes (2-way) | OCR'd one doc + opened SimplifyDrive |
| SimplifyTalent | Enterprise, Self-serve | Yes (2-way) | Completed one assessment + viewed report |
| SimplifyInsights | Enterprise, Self-serve | Yes (2-way) | Asked a question vs 2 sample companies |
| SimplifyHiring | Enterprise, Vendor, Candidate | Lead enterprise + invite link | Completed one full hiring cycle |

**Free trial = scoped functionality, not capped full product.** Each trial unlocks one or
two end-to-end flows containing the "aha" moment, with **20 credits within scope**. The
upgrade trigger is *needing more scope/volume*, not a crippled preview.

---

## 3. The adjusted master journey

```
Homepage: "What would you like to Simplify today?"
  ↓  Problem cards (product names secondary)
  ↓
[single-audience product] → straight into that flow
[multi-audience product]  → "Use for my whole team / Just for me, right now"
  ↓
SELF-SERVE:  act first → see one result (value moment) → register at the value wall
             → verify email before first sensitive action → free trial → pay
ENTERPRISE:  try it now (sandbox/sample) → capture use case + company profile
             → Book demo / Request POC → verify at contracting
  ↓
Activation (named event per product) → conversion → SSO carries across the suite
```

### Identity capture vs. verification (the key discipline)

| Stage | What we KNOW | What is DEFERRED |
|---|---|---|
| Anonymous visit | Company (reverse-IP), behaviour, source/campaign, geo, device | Everything personal |
| First action (pre-register) | + intent signal (what they tried) | Personal identity |
| Registration (the value wall) | Name, work email, company (from domain), role, mobile | Email/mobile confirmation |
| Verification | Confirmed email / domain / KYC | — |

We know *who is in the funnel* (company-level, behavioural) long before any individual
verifies. Verification timing moves; visibility does not.

---

## 4. Two motions, one infrastructure

The dividing line is economic: below ~$1k/yr self-serve is essential; above ~$10k/yr
guided onboarding pays for itself. Treat them as **separate journeys sharing infrastructure**.

| | Self-serve motion | Sales-led motion |
|---|---|---|
| Products | Legal, DocFlow, Talent, Insights, Studio, Credit (individual) | Transformer; team tiers of the rest |
| First value | Act before register; result in 2–5 min | Seeded demo / POC on sample data |
| Registration | At the value wall | Company profile up front (expected B2B) |
| Verification | After first value (email link) | At POC / contracting |
| Guidance | In-product checklist + behavioral email | Shared onboarding plan + human (CSM/sales) |
| Activation owner | The product | Product + a person nudging in parallel |

> **Caution (from CX doc):** don't pile every mechanic onto every product. More flows lift
> activation short-term and often hurt retention. Match mechanic to motion; pair every
> activation metric with a **day-30 retention cohort**.

---

## 5. CTA framework (commitment ladder + three-zone embed)

Four actions exist but are **not peers**. One primary CTA per surface, chosen by motion.

| CTA | Buyer signal | Motion / stage |
|---|---|---|
| Buy / Activate ("Try free") | "I'm ready now." | Self-serve |
| Request POC ("Run a proof of concept on your data") | "Prove it on my data." | Enterprise, mid-funnel |
| Request Demo ("See it on your use case") | "Show me it works." | Enterprise, early–mid |
| Talk to Sales ("Get pricing & security answers") | "I have a blocker." | Any stage |

**Three-zone embed per product page:**
- **Zone 1 — Hero:** one primary CTA only (highest-intent action for that motion).
- **Zone 2 — After first value:** contextual upgrade as a **card prompt** (cards beat banners).
- **Zone 3 — Persistent quiet escape hatch:** "Talk to Sales" in footer / slim sticky link.

**Name the value, not the format** — "Book Demo" → "See it on your use case"; "Request POC"
→ "Run a proof of concept on your data"; "Talk to Sales" → "Get pricing & security answers".

---

## 6. Activation layer (where best-in-class onboarding is actually measured)

For every product: name the single **activation event** (§2), design the first session to
reach it fast, and kill the blank canvas with **seeded sample data / templates** (the seed
data is the same scope the trial unlocks, so first-run and trial are one continuous
experience). Then layer:

- **Goal-based checklists** — 3–5 meaningful outcomes (not a feature tour), first step a quick win, persistent across sessions, optional tasks visually separated.
- **Behavioral email (stall recovery)** — fires on behaviour, not calendar:
  - Signed up, no first action → 60-second value email.
  - Saw one result, didn't return → reinforce unfinished value.
  - Hit free-credit limit → card-based upgrade prompt.
  - Enterprise demo no-show → reschedule + self-serve sample.

---

## 7. Identity & authorization spine (Zitadel + Cerbos)

Clean split: **Zitadel = who you are + org/roles**; **Cerbos = may this user do this action
on this resource**.

| Layer | Maps to | Effect |
|---|---|---|
| Zitadel Organization | One per customer (the tenant) | Isolates each customer's users & data |
| Zitadel Project | One shared **"Simplify Suite"** | All 8 products share one security context |
| Applications | The 8 products + their APIs | Share roles + the same login session |
| Project roles | Suite-wide + per-product | Asserted into the token for every product |
| Cerbos policies | Per-product, per-resource | Fine-grained allow/deny beyond RBAC |

**SSO is a consequence of this structure, not per-product work.** A user authenticates once;
session + role claims carry across every entitled product. Moving SimplifyLegal →
SimplifyInsights needs no second login.

> **Important nuance for this region:** The CX doc models **one Zitadel org per customer
> (tenant)**. That refines the earlier "one global org" idea — keep the existing
> [ZITADEL_ORG_DECOUPLING](../ZITADEL_ORG_DECOUPLING.md) invariant (login never derives
> tenant; membership via `tenant_members`). See backend plan §3 for the reconciliation and
> the open question.

### Roles (keep the set small)

Suite-wide: **Org Admin, Billing Admin, Member, Viewer, External/Vendor**.
Per-product: e.g. `legal.admin/reviewer/member`, `doc.admin/operator/viewer`,
`hiring.admin/recruiter/vendor/candidate`, `credit.admin/analyst/auditor`, etc. A user can
hold a suite role plus different per-product roles.

### Two access patterns to honor
- **Hiring vendor (external, scoped):** Org Admin generates a scoped vendor invite → vendor authenticates with own identity → token carries `hiring.vendor` + assigned-job metadata → Cerbos allows only those jobs.
- **Credit auditor (read-only, jurisdiction-masked):** token carries `credit.auditor` + jurisdiction → Cerbos allows view, denies decisions, masks out-of-jurisdiction fields.

---

## 8. SimplifyCore relationship (provisioning, subscription, billing)

SimplifyCore is the platform layer beneath all products: provisioning, subscriptions,
entitlements, metering (Simplify Token/Credit), billing, payments. **Products never store
billing state — they read entitlement from Core.** One source of truth, two surfaces
(per-product subscription page + Core billing centre are two views of the same data).

Onboarding-module touchpoints:
- On self-serve checkout / enterprise provisioning, **Core** grants the org access to product apps in the Suite project (Zitadel) and writes entitlement.
- **Cerbos** receives plan/entitlement state as attributes → can deny when a plan lapses or credits run out, not just on role.
- The onboarding module **reads** entitlement to decide "register at value wall" vs "trial" vs "locked", and to render the right CTA — it does not own that state.

---

## 9. Enterprise provisioning & "from sold to live"

```
Demo/POC success → plan + contract agreed
  → Zitadel Organization provisioned (sales-assisted OR verified self-signup)
  → First user granted ORG_OWNER → becomes Org Admin
  → Admin (self-service in Zitadel): configures SSO/MFA, invites users, assigns roles
  → Suite project granted to the org with purchased products' roles
  → Users accept invite → set auth method → SSO into entitled products
  → Cerbos enforces per-product, per-resource access
  → Vendors added as external users, scoped to assigned workspaces
  → Org is live
```

Two provisioning models: **sales-assisted** (large/contract-tied — team creates workspace,
invites champion as Org Admin) and **verified self-signup** (smaller — work-email domain
verified → workspace pending plan).

---

## 10. Homepage & information architecture

Primary surface = the **problem**, product names secondary:

| User intent (shown first) | Product (secondary) |
|---|---|
| Build software faster | SimplifyStudio |
| Modernize legacy systems | SimplifyTransformer |
| Review a legal document | SimplifyLegal |
| Hire talent faster | SimplifyHiring |
| Automate document processing | SimplifyDocFlow |
| Assess skills and careers | SimplifyTalent |
| Generate business insights | SimplifyInsights |
| Assess credit risk | SimplifyCredit |

Keep **audience-first** (Enterprise / Individual / Vendor / Candidate) as *secondary* nav
for visitors who already know their role, plus campaign landing pages.

---

## 11. Compliance / trust fixes baked in (from CX doc)

- [ ] **Consent is opt-in** — privacy/contact checkbox **unchecked by default** everywhere (GDPR; a pre-ticked box is not valid consent). The new mock already shows this.
- [ ] **Form title matches the CTA clicked** (a "POC" button must not land on a "Request your demo" title).
- [ ] **Fix "Simlify" typo** + replace placeholder phone data with validated input.
- [ ] **Data residency surfaced early** (ID · SG · IN · AE) on enterprise surfaces.
- [ ] **Real applicant data (KTP/SLIK/BPKB)** requires accepting data-handling terms at first real upload — trial runs on sample data until then.

---

## 12. Module map & where each part lives

```
simplify-onboarding/
  00_MASTER_PLAN.md                      ← this file
  SIMPLIFY_ONBOARDING_BACKEND_PLAN.md
  SIMPLIFY_ONBOARDING_FRONTEND_PLAN.md
  simplify-onboarding-service/           ← Go backend (when we write code)
  simplify-onboarding-web/               ← React portal + homepage + product pages
  Simplify_Onboarding_CX.pdf             ← CX source (keep for reference)
```

---

## 13. Phased delivery

| Phase | Goal | Outcome | Status |
|---|---|---|---|
| **P0 — Foundation** | Zitadel project/apps + IdPs; scaffold service & web; analytics + reverse-IP wiring | Anonymous identity + SSO substrate | 🟡 service+web scaffolded, visitor/funnel endpoints built; Suite project/per-product OIDC apps + analytics/reverse-IP pending |
| **P1 — Auth backbone (DocFlow pilot)** | Branded create/sign-in + email OTP; DocFlow as first client; session proven | One account works end-to-end | 🟡 branded create/sign-in + Zitadel email-OTP + logged-in account state **done & verified**; DocFlow connected at **shared-account** level (same Zitadel org + launch link); silent OIDC SSO pending |
| **P2 — Value-first self-serve** | Homepage problem-router + product landing pages with try → value wall → register; seeded first-run | First value before the gate | 🟡 homepage problem-router + product landing (motion split, CTA) **done**; try-before-register value wall + seeded first-run pending |
| **P3 — Second product + silent SSO** | Onboard Legal/Drive; prove no second login; single-logout | True cross-product SSO | ⬜ pending (needs OIDC bridge) |
| **P4 — Activation + CTAs** | Checklists, behavioral-email triggers, three-zone CTAs, contextual upgrade cards | Activation/retention instrumented | ⬜ pending |
| **P5 — Enterprise motion** | Product landing "team" path, demo/POC flows, sales-assisted provisioning, Org Admin user mgmt | Sales-led journey live | ⬜ pending |
| **P6 — SimplifyCore + Cerbos depth** | Entitlement reads, plan-aware gating, per-product Cerbos policies, deferred mobile/KYC | Plan-aware authz across suite | 🟡 entitlement stub + deferred mobile verify built; Cerbos + real Core pending |
| **P7 — Roll remaining products** | All 8 onboarded; rename Zitadel org; decommission per-product hosted logins | Suite-wide onboarding | ⬜ pending |

---

## 14. Success metrics (instrument from P2)

- Activation rate per product (the named event) + **day-30 retention cohort** alongside it.
- Self-serve: time-to-first-value (target 2–5 min), value-wall registration rate, trial→paid.
- Enterprise: company-identified visits, sandbox→demo/POC rate, POC→close.
- Drop-off by stage (anonymous → first action → register → verify → pay).

---

## 15. Open questions for the manager (consolidated)

- [ ] **Org model:** one Zitadel org per *customer/tenant* (CX doc) vs one global org (current). Recommend per-customer for enterprise isolation; confirm.
- [ ] Final auth/onboarding domain + brand name on the portal ("Simplify").
- [ ] Which **SMS provider** for +62 mobile OTP; budget.
- [ ] Is **SimplifyCore** already built / in progress, or do we stub entitlement reads for now?
- [ ] Reverse-IP / company-ID vendor (Clearbit Reveal / RB2B / Albacross / Leadfeeder) + analytics (PostHog?) + CRM choice.
- [ ] Which products exist as deployable apps today vs greenfield (affects P3/P7 ordering).
- [ ] Launch passkeys/MFA in P1 or defer (mock lists "MFA & passkeys built in").
- [ ] Is "Company domain verified" a real B2B allow-list or cosmetic? Source of allowed domains.
