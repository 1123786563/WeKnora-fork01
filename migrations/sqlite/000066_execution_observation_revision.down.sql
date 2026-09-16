DROP INDEX IF EXISTS idx_execution_observations_revision;
-- SQLite cannot drop a column on older supported runtimes; the additive field is retained on rollback.
