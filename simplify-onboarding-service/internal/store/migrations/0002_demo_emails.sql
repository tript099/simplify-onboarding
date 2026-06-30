-- Track the MailForge message ids for the emails sent on each demo/POC request,
-- and a last-updated timestamp. Status transitions to 'notified' once emails go out.
ALTER TABLE demo_requests
    ADD COLUMN IF NOT EXISTS confirmation_email_id text,
    ADD COLUMN IF NOT EXISTS notify_email_id       text,
    ADD COLUMN IF NOT EXISTS updated_at            timestamptz NOT NULL DEFAULT now();
