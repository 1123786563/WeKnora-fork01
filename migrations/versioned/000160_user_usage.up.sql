DO $$ BEGIN RAISE NOTICE '[Migration 000160] user_usage'; END $$;
CREATE TABLE IF NOT EXISTS user_usage (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    user_id VARCHAR(512) NOT NULL DEFAULT '',
    window_start DATE NOT NULL,
    model VARCHAR(128) NOT NULL DEFAULT '',
    flow VARCHAR(32) NOT NULL DEFAULT '',
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens BIGINT NOT NULL DEFAULT 0,
    cache_write_tokens BIGINT NOT NULL DEFAULT 0,
    cost_microcredits BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_user_usage_dims UNIQUE (tenant_id, user_id, window_start, model, flow)
);
CREATE INDEX IF NOT EXISTS idx_user_usage_tenant_window ON user_usage (tenant_id, window_start);
