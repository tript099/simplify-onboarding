# Demo / POC Request Flow — Implementation Plan

> Turn the "Book a demo / Request a POC / Talk to sales" forms into a complete,
> production-grade pipeline: **persist → confirm to the requester → notify the Simplify
> team → schedule a Microsoft Teams meeting**, using the shared **MailForge** email
> service (the same one DocFlow uses) with its own API key and branded templates.
>
> Status: **✅ Phase 1 SHIPPED & verified** (persist + confirm + team-notify via a dedicated
> MailForge project key). Phase 2 (admin/recipients UI) pending. Phase 3 (Teams scheduling)
> deferred — user has their own scheduler. Deviation from plan: team recipients are a JSON
> file (per user decision), not a DB table.

---

## ✅ Completed checklist (Phase 1)

- [x] Migration `0002` — `confirmation_email_id`, `notify_email_id`, `updated_at` on `demo_requests`
- [x] MailForge client ported into onboarding (`internal/mailforge`)
- [x] `notify` service — render + send, async, idempotent, fail-soft (`internal/notify`)
- [x] Two branded HTML templates — requester confirmation + team notification
- [x] Team recipients from a JSON file, routable by type/product, hot-reloaded (`config/demo-recipients.json`)
- [x] `SubmitDemo` wired: persist → send both emails → store message ids → status `new→notified`
- [x] Config + compose env (`MAILFORGE_*`, recipients mount, `host.docker.internal`)
- [x] Frontend success-state copy ("we've emailed a confirmation to …")
- [x] Dedicated MailForge project **"Simplify Onboarding"** + `email:send` key provisioned & wired
- [x] **Verified end-to-end** — request → confirmation + team-notify sent, ids stored, status `notified`
- [ ] _Pending you:_ replace placeholder team addresses in `demo-recipients.json` with real inboxes

---

## 1. Goals

1. Every demo/POC/contact request is **durably persisted** (already partly done — the
   `demo_requests` table exists). Add a proper status lifecycle + scheduling fields.
2. The **requester** receives a branded confirmation email immediately.
3. The **Simplify team** receives a notification email — to a **configurable** list of
   recipients (per request type and/or per product).
4. Email is sent through **MailForge** (central email service) using a **dedicated API
   key for the onboarding project** and **beautiful, branded templates**.
5. Demo/POC **timings are scheduled as Microsoft Teams meetings** (with calendar invites
   to both the requester and the assigned rep).

### Non-goals (this iteration)
- A full CRM. We capture the pipeline; deep CRM sync (HubSpot/Salesforce) is a later hook.
- An internal admin SPA. We expose admin **APIs**; a UI is a follow-up.

---

## 2. Current state

- **Form** (`simplify-onboarding-web`): `BookDemo.tsx` collects demo/poc/contact with
  use-case, company, scheduling preferences, consent. On submit → `POST /onb/demo`.
- **Backend** (`simplify-onboarding-service`): `SubmitDemo` persists to the
  `demo_requests` table (Postgres, owned by onboarding) — `store.InsertDemoRequest`.
- **Missing:** no email to anyone, no team-recipient config, no scheduling.

Existing `demo_requests` columns: `id, type, status('new'), product_key, first_name,
last_name, email, phone, job_title, company, company_size, industry, country, use_case,
timeline, budget, notes, message, preferred_date, time_slot, timezone, consent, payload,
created_at`.

---

## 3. Target end-to-end flow

```
 Requester                Onboarding API            Onboarding DB        MailForge        Microsoft Graph
    │  submit form            │                          │                  │                 │
    │────────────────────────►│ POST /onb/demo           │                  │                 │
    │                         │ 1. INSERT demo_request ─►│ (status=new)     │                 │
    │                         │ 2. enqueue notifications │                  │                 │
    │   200 { requestId } ◄───┤    (async, best-effort)  │                  │                 │
    │                         │                          │                  │                 │
    │                         │ 3a. send CONFIRMATION ───────────────────►  │ /v1/send        │
    │  ✉ "We got your request"◄──────────────────────────────────────────  │ (template)      │
    │                         │ 3b. send TEAM NOTIFY ────────────────────►  │ /v1/send        │
    │                         │     (to configured recipients)              │                 │
    │   Simplify team  ✉ "New POC request: Acme" ◄──────────────────────── │                 │
    │                         │                          │                  │                 │
    │   ── later: rep confirms a time (admin API or self-schedule link) ──  │                 │
    │                         │ 4. create Teams meeting ───────────────────────────────────►  │ POST /onlineMeetings
    │                         │ 5. UPDATE demo_request (scheduled_at, meeting_url) ─►│         │
    │                         │ 6. send INVITE (.ics + Teams link) ───────►  │ /v1/send        │
    │  ✉ calendar invite ◄────────────────────────────────────────────────  │                 │
```

Phases (Section 11) make 1–3 the first deliverable; 4–6 follow.

---

## 4. Data model changes

Add a migration `000X_demo_pipeline.sql` to the onboarding DB.

### 4.1 Extend `demo_requests`

```sql
ALTER TABLE demo_requests
  ADD COLUMN assigned_to        text,              -- rep email handling it
  ADD COLUMN scheduled_at       timestamptz,       -- confirmed meeting time
  ADD COLUMN meeting_url        text,              -- Teams join link
  ADD COLUMN meeting_id         text,              -- Graph onlineMeeting id
  ADD COLUMN confirmation_email_id text,           -- MailForge message id (requester)
  ADD COLUMN notify_email_id    text,              -- MailForge message id (team)
  ADD COLUMN updated_at         timestamptz NOT NULL DEFAULT now();
```

Status lifecycle (already has `status` default `new`):
`new → notified → contacted → scheduled → completed → (won|lost)`.

### 4.2 Configurable team recipients

```sql
CREATE TABLE demo_notification_recipients (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email       text NOT NULL,
    name        text,
    -- routing filters: NULL = matches all
    request_type text,             -- 'demo' | 'poc' | 'contact' | NULL (all)
    product_key  text,             -- 'docflow' | … | NULL (all)
    active       boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now()
);
```

> Why a table, not env? The user explicitly wants the team list **configurable** without
> redeploys, and routable per product/type (e.g. POCs → solutions-eng, contact → sales).
> Seed it from an env default on first boot so it works out of the box.

---

## 5. Email via MailForge

### 5.1 Provision a dedicated API key (one-time, infra step)

MailForge is **project-scoped**. We create/issue a key for the **onboarding** project,
using an existing **admin key** (DocFlow's / infra's `admin:keys` scope) — exactly as the
user requested ("create a mail service API key for this project using DocFlow's key").

Steps (documented as a runbook, executed once):
1. With an admin key, create a project **"Simplify Onboarding"** (or reuse a shared
   "Simplify" project) in the email service.
2. Issue an API key with scopes: **`email:send`, `template:read`, `template:write`**.
3. Store it as the onboarding secret **`MAILFORGE_API_KEY`** (`mf_proj_…`).

Config on the onboarding service:

| Env | Value |
|---|---|
| `MAILFORGE_URL` | `http://email-service-api:8080` (internal) |
| `MAILFORGE_API_KEY` | `mf_proj_…` (the new key) |
| `MAILFORGE_FROM_NAME` | `Simplify` |
| `MAILFORGE_FROM_EMAIL` | `no-reply@simplifyaipro.com` |
| `MAILFORGE_REPLY_TO` | `sales@simplifyaipro.com` |

> Note on standalone-ness: MailForge, like Zitadel, is **shared infrastructure** accessed
> over HTTP with an API key — not a product's private DB. Depending on it does **not**
> break the "standalone" property (no shared datastore coupling).

### 5.2 MailForge client in onboarding

Port `docflow-bff/internal/mailforge/client.go` into
`simplify-onboarding-service/internal/mailforge/` (Bearer auth, `POST /v1/send` with
`template_id` + `variables`). Add a thin `internal/notify` service that builds the
`SendRequest`s and calls the client.

Send is **async + best-effort + idempotent**:
- Fire from a goroutine after the request is persisted (never block the form response).
- Set `idempotency_key = "demo-confirm:" + requestId` / `"demo-notify:" + requestId` so a
  retry can't double-send.
- On failure, log + (Phase 2) enqueue a retry in Redis; the request is already saved.

### 5.3 Templates to create (under the new key)

Each is a MailForge **template collection + published version** with `{{.var}}`
placeholders. We create them via `POST /v1/templates` + `/versions` (template:write), then
reference by `template_id`.

| Template | Recipient | Trigger | Key variables |
|---|---|---|---|
| `onb-demo-confirmation` | Requester | on submit | `first_name, product_name, request_type_label, summary_lines, expected_reply_window` |
| `onb-demo-team-notify` | Simplify team | on submit | `request_type, product_name, company, contact_name, contact_email, phone, use_case, timeline, budget, preferred_slot, admin_link` |
| `onb-demo-invite` | Requester + rep | on schedule | `first_name, product_name, meeting_time, timezone, teams_join_url, rep_name` |

### 5.4 Template design (beautiful + on-brand)

- **Layout:** single-column, max-width 600px, system font stack, light background, white
  card, generous padding. Mobile-safe (inline CSS, table-based for Outlook).
- **Header:** Simplify logo + product accent bar (per `product_key` accent).
- **Confirmation:** friendly headline ("We've got your request, {{.first_name}}"), a
  recap card of what they asked for, a "what happens next" 3-step strip (mirrors the
  portal's BookDemo aside), trust line ("reply within 24h · no card"), and a "while you
  wait → Try it now" CTA (ties into the demo-account flow).
- **Team notify:** dense, scannable — request type badge, company, contact, use-case,
  preferred time, and a deep link to the admin record. `reply_to` set to the requester so
  the rep can reply directly.
- **Invite:** the meeting time prominently, a big "Join Microsoft Teams" button, add-to-
  calendar, and the rep's name. Attach an **`.ics`** so it lands in any calendar.

> Build templates as standalone HTML first, preview via `POST /v1/templates/{id}/versions/{v}/preview`
> with sample variables, iterate, then publish. Keep a copy of the source HTML in the repo
> under `simplify-onboarding/email-templates/` for version control.

---

## 6. Microsoft Teams scheduling

Three approaches, in increasing control/effort. **Recommendation: ship B (self-schedule
link) first for speed, add A (Graph) for full control.**

### A. Microsoft Graph — create a real Teams meeting (full control)
- App registration (Azure AD) with **application** permission `OnlineMeetings.ReadWrite.All`
  (+ a service mailbox / application access policy), or `Calendars.ReadWrite` to also drop
  a calendar event.
- Flow: rep confirms a slot (admin API) → onboarding calls Graph
  `POST /users/{organizer}/onlineMeetings` (or `/events` with `isOnlineMeeting:true`) →
  store `meeting_url`/`meeting_id` → email the invite (§5.3 `onb-demo-invite`) with `.ics`.
- Pros: native Teams meeting, no third-party. Cons: needs Graph app perms + a service
  organizer mailbox.

### B. Self-scheduling link (fastest, least infra)
- Put a **Microsoft Bookings** (or Calendly-for-Teams) link in the confirmation email so
  the requester picks a slot themselves → Bookings auto-creates the Teams meeting + invites.
- Pros: zero Graph integration, requester self-serves. Cons: depends on a Bookings page.

### C. `.ics` calendar invite with a Teams link (universal fallback)
- Generate an `.ics` (VEVENT) with a pre-created Teams link and email it. Works in any
  calendar client. Good as the **delivery mechanism** for A.

> Decision needed (Section 13): confirm whether infra can grant the Graph app permissions
> + a service organizer mailbox. If yes → A. If not now → B, with C as the invite format.

---

## 7. Backend implementation

### 7.1 `SubmitDemo` (extend existing)
1. Validate + `InsertDemoRequest` (existing).
2. Resolve recipients: `SELECT … FROM demo_notification_recipients WHERE active AND
   (request_type IS NULL OR request_type=$type) AND (product_key IS NULL OR product_key=$key)`.
   Fallback to `MAILFORGE_DEFAULT_TEAM` env if the table is empty.
3. Spawn goroutine: send `onb-demo-confirmation` to requester + `onb-demo-team-notify` to
   recipients (idempotent). Persist returned `confirmation_email_id`/`notify_email_id`;
   set status → `notified`.
4. Return `{ ok, requestId }` immediately (unchanged contract).

### 7.2 Admin endpoints (internal-secret protected)
```
GET   /onb/admin/demo-requests           # list/filter (status, type, product, date)
GET   /onb/admin/demo-requests/{id}      # detail
PATCH /onb/admin/demo-requests/{id}      # update status / assigned_to
POST  /onb/admin/demo-requests/{id}/schedule  # {slot, organizer} → Teams meeting + invite
GET/POST/DELETE /onb/admin/notify-recipients  # manage the team recipient list
```
Guard with the existing internal-secret / an admin token (consistent with the platform's
other internal endpoints). These back a future admin UI.

### 7.3 `notify` service
- `SendConfirmation(ctx, req) (msgID, err)`
- `SendTeamNotify(ctx, req, recipients) (msgID, err)`
- `SendInvite(ctx, req, meeting) (msgID, err)`
Each builds a `mailforge.SendRequest{ From, To, ReplyTo, TemplateID, Variables, IdempotencyKey, Tags }`.

---

## 8. Frontend

- **BookDemo success state:** update copy to "Check your inbox — we've emailed a
  confirmation to {{email}}" (reinforces the email was sent). Already has the success card.
- **(Optional, later) Admin view:** a simple table for the team to see requests + mark
  status — consumes `/onb/admin/demo-requests`.
- No change to the submission contract.

---

## 9. Security, reliability, observability

- **Idempotency:** `idempotency_key` per email so retries don't double-send.
- **Async + fail-soft:** email/scheduling never block or fail the user's submission; the
  request is persisted first. Failures are logged + retriable.
- **PII:** request payloads contain personal data — already in our own DB; ensure logs
  don't dump full payloads; restrict admin endpoints.
- **Rate limit** `POST /onb/demo` (per-IP) to prevent form spam; add a honeypot field +
  optional hCaptcha for public exposure.
- **Observability:** structured logs with `request_id`, MailForge message ids, and
  metrics (requests submitted, emails sent/failed, meetings scheduled).
- **Reply-to:** team-notify `reply_to` = requester so reps reply directly; confirmation
  `reply_to` = `sales@`.

---

## 10. Testing

- Unit: recipient resolution (routing filters), template variable mapping, idempotency.
- Integration: against the email service in a test project key — assert `/v1/send` 202 +
  message id; preview templates with sample vars.
- E2E: submit each request type → assert DB row + two emails (use a mailbox/inbox capture)
  → schedule → assert meeting fields + invite.
- Failure paths: MailForge down (request still saved, status stays `new`, retry works);
  no recipients configured (falls back to default).

---

## 11. Phasing / milestones

| Phase | Scope | Outcome |
|---|---|---|
| ✅ **P1 — Persist + Confirm + Notify** | Migration (status/fields), MailForge client + `notify` service, 2 templates (confirmation, team-notify), wire into `SubmitDemo`, recipients from JSON file, success-state copy. | **DONE & verified** — requests saved and **both emails go out**. |
| ⬜ **P2 — Admin + Recipients config** | Admin endpoints (list/detail/status/assign), recipients UI. | Team can triage; recipients editable from UI. |
| ⏸️ **P3 — Scheduling** | Teams meeting + `onb-demo-invite` template + `.ics`, schedule endpoint. | **Deferred** — user has their own Teams scheduler; will integrate via its API endpoint. |

Each phase is independently shippable and leaves the system working.

---

## 12. Files added/changed (P1) — ✅ done

```
simplify-onboarding-service/
  internal/store/migrations/0002_demo_emails.sql      ✅ email-id columns + updated_at
  internal/store/demo.go                              ✅ +UpdateDemoEmails (msg ids + status)
  internal/mailforge/client.go                        ✅ ported MailForge client
  internal/notify/notify.go                           ✅ builds + sends the emails
  internal/notify/recipients.go                       ✅ JSON recipients loader (hot-reload)
  internal/notify/templates/*.html                    ✅ 2 embedded branded templates
  internal/handler/onboarding.go                      ✅ SubmitDemo: persist → notify (async)
  internal/config/config.go                           ✅ MAILFORGE_* + DEMO_RECIPIENTS_FILE
  docker-compose.yml                              ✅ MAILFORGE_* env, recipients mount, host.docker.internal
  config/demo-recipients.json                         ✅ team recipients (placeholder addrs)
  scripts/create-mailforge-project.sh                 ✅ provision dedicated project + key
simplify-onboarding-web/src/routes/BookDemo.tsx       ✅ success-state copy
```

Networking: onboarding reaches MailForge at `http://host.docker.internal:8099` (extra_hosts
added). The dedicated project key is in `ONBOARDING_MAILFORGE_API_KEY`.

---

## 13. Decisions — ✅ resolved

1. ✅ **MailForge project:** dedicated **"Simplify Onboarding"** project created (id `d57439a2`) +
   `email:send` key — wired as `ONBOARDING_MAILFORGE_API_KEY`.
2. ⏸️ **Teams scheduling:** deferred — user has their own scheduler; will integrate via its API.
3. 🟡 **Team recipients:** sourced from `config/demo-recipients.json` (routable by type/product),
   UI-configurable later. **Still placeholder addresses — user to set real inboxes.**
4. ✅ **From/Reply-to identities:** `no-reply@simplifyaipro.com` (from) / `sales@simplifyaipro.com`
   (reply-to) — confirmed OK, sends verified.

---

*Pro note: P1 delivers the user-visible value (saved + confirmation + team notified) with
the smallest surface and no hard dependency on Graph permissions. Scheduling (P3) is
deliberately decoupled so a permissions delay never blocks the email pipeline.*
