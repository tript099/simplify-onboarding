-- Meeting scheduling: when a demo/POC is scheduled, store the Google Meet / Teams
-- join URL and the confirmed time. Status moves to 'scheduled'.
ALTER TABLE demo_requests
    ADD COLUMN IF NOT EXISTS meeting_url   text,
    ADD COLUMN IF NOT EXISTS scheduled_at  timestamptz,
    ADD COLUMN IF NOT EXISTS invite_email_id text;
