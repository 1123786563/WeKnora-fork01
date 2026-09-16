ALTER TABLE execution_cleanup_artifacts DROP COLUMN IF EXISTS delete_lease_until;
ALTER TABLE execution_cleanup_artifacts DROP COLUMN IF EXISTS delete_token;
