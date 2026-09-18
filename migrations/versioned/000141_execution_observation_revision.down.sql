DROP INDEX IF EXISTS idx_execution_observations_revision;
ALTER TABLE execution_observations DROP COLUMN IF EXISTS deletion_revision;
