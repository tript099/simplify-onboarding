-- Capture the "How will you use this?" pick on demo/POC/contact requests:
-- 'self_serve' (just for me) or 'team' (whole team). Just an input we store.
ALTER TABLE demo_requests
    ADD COLUMN IF NOT EXISTS usage text;
