-- SQLite dialect of PG 000158
CREATE TABLE IF NOT EXISTS message_feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    user_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    session_id TEXT NOT NULL DEFAULT '',
    rating TEXT NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_message_feedback UNIQUE (message_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_message_feedback_session ON message_feedback (session_id);
CREATE INDEX IF NOT EXISTS idx_message_feedback_tenant_time ON message_feedback (tenant_id, created_at);
