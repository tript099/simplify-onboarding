-- Onboarding service schema (v1).
-- This database is owned by the onboarding service. The shared platform DB
-- (app_users/tenants) remains the source of truth for cross-product identity; here
-- we keep the richer onboarding profile, the sales pipeline, funnel analytics and an
-- auth audit trail.

-- ── users ────────────────────────────────────────────────────────
-- One row per account that signs up / signs in, synced from Zitadel on every login.
-- The UNIVERSAL identity is zitadel_sub (the IdP subject) — it's stable across every
-- product. Each product maps that sub to its OWN internal id; this service is product
-- agnostic and never stores another product's id. `id` is this DB's own key.
CREATE TABLE IF NOT EXISTS users (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    zitadel_sub      text UNIQUE NOT NULL,
    email            text NOT NULL,
    first_name       text,
    last_name        text,
    display_name     text,
    phone            text,
    company          text,
    job_title        text,
    -- NB: no role/RBAC here. This is a universal account layer; roles are owned by
    -- each downstream product (e.g. DocFlow resolves tenant roles in its own DB).
    auth_method      text,           -- password | otp | google | microsoft | demo
    email_verified   boolean NOT NULL DEFAULT false,
    phone_verified   boolean NOT NULL DEFAULT false,
    is_demo          boolean NOT NULL DEFAULT false,
    login_count      bigint  NOT NULL DEFAULT 0,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    last_login_at    timestamptz
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users (lower(email));

-- ── demo_requests ────────────────────────────────────────────────
-- Sales-led requests: "Book a demo", "Request a POC", "Talk to sales".
CREATE TABLE IF NOT EXISTS demo_requests (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    type          text NOT NULL DEFAULT 'demo',   -- demo | poc | contact
    status        text NOT NULL DEFAULT 'new',     -- new | contacted | qualified | won | lost
    product_key   text,
    first_name    text,
    last_name     text,
    email         text,
    phone         text,
    job_title     text,
    company       text,
    company_size  text,
    industry      text,
    country       text,
    use_case      text,
    timeline      text,
    budget        text,
    notes         text,
    message       text,
    preferred_date text,
    time_slot     text,
    timezone      text,
    consent       boolean NOT NULL DEFAULT false,
    payload       jsonb,                           -- full raw submission
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_demo_requests_type ON demo_requests (type);
CREATE INDEX IF NOT EXISTS idx_demo_requests_email ON demo_requests (lower(email));
CREATE INDEX IF NOT EXISTS idx_demo_requests_created ON demo_requests (created_at DESC);

-- ── onboarding_events ────────────────────────────────────────────
-- Value-first funnel analytics: anonymous visitor + (once known) user actions.
CREATE TABLE IF NOT EXISTS onboarding_events (
    id          bigserial PRIMARY KEY,
    visitor_id  text,
    user_id     text,
    event       text NOT NULL,
    product_key text,
    metadata    jsonb,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_events_visitor ON onboarding_events (visitor_id);
CREATE INDEX IF NOT EXISTS idx_events_event ON onboarding_events (event);
CREATE INDEX IF NOT EXISTS idx_events_created ON onboarding_events (created_at DESC);

-- ── login_audit ──────────────────────────────────────────────────
-- Security/auth trail: who signed in, how, from where.
CREATE TABLE IF NOT EXISTS login_audit (
    id          bigserial PRIMARY KEY,
    user_id     text,
    email       text,
    auth_method text,                  -- password | otp | google | microsoft | demo
    success     boolean NOT NULL DEFAULT true,
    ip          text,
    user_agent  text,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_login_audit_user ON login_audit (user_id);
CREATE INDEX IF NOT EXISTS idx_login_audit_created ON login_audit (created_at DESC);
