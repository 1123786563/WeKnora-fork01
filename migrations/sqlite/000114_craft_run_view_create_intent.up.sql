-- A non-null intent means a non-idempotent session create may already have
-- been sent. Existing rows predate this fence, so treat them conservatively
-- as attempted instead of granting an unsafe retry permission.
ALTER TABLE craft_run_views
    ADD COLUMN session_create_intent_at DATETIME NULL;

UPDATE craft_run_views
SET session_create_intent_at = updated_at
WHERE session_create_intent_at IS NULL;
