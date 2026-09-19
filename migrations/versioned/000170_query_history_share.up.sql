DO $$ BEGIN RAISE NOTICE '[Migration 000170] query_history_share'; END $$;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS share_token VARCHAR(64) DEFAULT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_sessions_share_token ON sessions (share_token) WHERE share_token IS NOT NULL;

ALTER TABLE tenants ADD COLUMN IF NOT EXISTS query_history_config JSONB DEFAULT NULL;

CREATE TABLE IF NOT EXISTS query_history_export_jobs (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    requested_by VARCHAR(512) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    file_path VARCHAR(512) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_query_history_export_jobs_tenant ON query_history_export_jobs (tenant_id);
