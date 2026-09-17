ALTER TABLE execution_cleanup_artifacts DROP COLUMN IF EXISTS receipt_state;
ALTER TABLE execution_cleanup_artifacts DROP COLUMN IF EXISTS attempt;
ALTER TABLE execution_cleanup_artifacts DROP COLUMN IF EXISTS provider_idempotency_key;
