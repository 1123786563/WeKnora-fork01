DROP INDEX IF EXISTS idx_query_history_export_jobs_tenant;
DROP TABLE IF EXISTS query_history_export_jobs;

ALTER TABLE tenants DROP COLUMN IF EXISTS query_history_config;

DROP INDEX IF EXISTS uq_sessions_share_token;
ALTER TABLE sessions DROP COLUMN IF EXISTS share_token;
