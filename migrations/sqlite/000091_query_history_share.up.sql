-- SQLite dialect of PG 000170_query_history_share:
-- share links on sessions, workspace query-history policy on tenants, and
-- the async export job table. SQLite supports partial indexes natively.

ALTER TABLE sessions ADD COLUMN share_token TEXT DEFAULT NULL;
-- '' excluded besides NULL — see the PG original (GORM zero-value inserts).
CREATE UNIQUE INDEX IF NOT EXISTS uq_sessions_share_token ON sessions (share_token) WHERE share_token IS NOT NULL AND share_token <> '';

ALTER TABLE tenants ADD COLUMN query_history_config TEXT DEFAULT NULL;

CREATE TABLE IF NOT EXISTS query_history_export_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    requested_by TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    file_path TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_query_history_export_jobs_tenant ON query_history_export_jobs (tenant_id);
