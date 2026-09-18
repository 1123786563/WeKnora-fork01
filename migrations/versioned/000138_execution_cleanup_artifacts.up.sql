CREATE TABLE execution_cleanup_artifacts (
    tenant_id BIGINT NOT NULL,
    session_id VARCHAR(64) NOT NULL,
    deletion_revision BIGINT NOT NULL,
    ref VARCHAR(2048) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    state VARCHAR(16) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    PRIMARY KEY (tenant_id, session_id, deletion_revision, ref),
    CHECK (state IN ('pending', 'deleted'))
);
CREATE INDEX idx_execution_cleanup_artifacts_ref ON execution_cleanup_artifacts (tenant_id, ref, state);
