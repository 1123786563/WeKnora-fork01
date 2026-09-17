ALTER TABLE execution_observations ADD COLUMN deletion_revision BIGINT NOT NULL DEFAULT 0;
CREATE INDEX idx_execution_observations_revision ON execution_observations (tenant_id, run_id, deletion_revision);
