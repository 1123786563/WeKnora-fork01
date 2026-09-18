DROP INDEX IF EXISTS idx_mobile_notification_next_attempt;
ALTER TABLE mobile_notification_intents DROP COLUMN IF EXISTS last_error;
ALTER TABLE mobile_notification_intents DROP COLUMN IF EXISTS receipt_id;
ALTER TABLE mobile_notification_intents DROP COLUMN IF EXISTS next_attempt_at;
