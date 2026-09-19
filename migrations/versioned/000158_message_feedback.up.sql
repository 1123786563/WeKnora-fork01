DO $$ BEGIN RAISE NOTICE '[Migration 000158] message_feedback'; END $$;
CREATE TABLE IF NOT EXISTS message_feedback (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    user_id VARCHAR(512) NOT NULL,
    message_id VARCHAR(36) NOT NULL,
    session_id VARCHAR(36) NOT NULL DEFAULT '',
    rating VARCHAR(16) NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_message_feedback UNIQUE (message_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_message_feedback_session ON message_feedback (session_id);
CREATE INDEX IF NOT EXISTS idx_message_feedback_tenant_time ON message_feedback (tenant_id, created_at);
