ALTER TABLE execution_cleanup_artifacts ADD COLUMN delete_token VARCHAR(2048) NOT NULL DEFAULT '';
ALTER TABLE execution_cleanup_artifacts ADD COLUMN delete_lease_until DATETIME;
CREATE INDEX idx_execution_cleanup_artifacts_delete_lease ON execution_cleanup_artifacts (delete_lease_until, state);
