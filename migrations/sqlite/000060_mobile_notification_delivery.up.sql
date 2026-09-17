ALTER TABLE mobile_notification_intents ADD COLUMN next_attempt_at DATETIME;
ALTER TABLE mobile_notification_intents ADD COLUMN receipt_id VARCHAR(256) NOT NULL DEFAULT '';
ALTER TABLE mobile_notification_intents ADD COLUMN last_error TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_mobile_notification_next_attempt ON mobile_notification_intents (state, next_attempt_at, expires_at, created_at);
