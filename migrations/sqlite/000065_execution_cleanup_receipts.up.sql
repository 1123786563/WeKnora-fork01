ALTER TABLE execution_cleanup_artifacts ADD COLUMN provider_idempotency_key VARCHAR(2048) NOT NULL DEFAULT '';
ALTER TABLE execution_cleanup_artifacts ADD COLUMN attempt INTEGER NOT NULL DEFAULT 0;
ALTER TABLE execution_cleanup_artifacts ADD COLUMN receipt_state VARCHAR(16) NOT NULL DEFAULT 'none';
CREATE INDEX idx_execution_cleanup_artifacts_provider_key ON execution_cleanup_artifacts (provider_idempotency_key);
