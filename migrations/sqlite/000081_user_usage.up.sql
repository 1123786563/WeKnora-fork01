-- SQLite dialect of PG 000160
CREATE TABLE IF NOT EXISTS user_usage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    user_id TEXT NOT NULL DEFAULT '',
    window_start DATE NOT NULL,
    model TEXT NOT NULL DEFAULT '',
    flow TEXT NOT NULL DEFAULT '',
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens BIGINT NOT NULL DEFAULT 0,
    cache_write_tokens BIGINT NOT NULL DEFAULT 0,
    cost_microcredits BIGINT NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_user_usage_dims UNIQUE (tenant_id, user_id, window_start, model, flow)
);
CREATE INDEX IF NOT EXISTS idx_user_usage_tenant_window ON user_usage (tenant_id, window_start);
