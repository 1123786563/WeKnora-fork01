CREATE TABLE execution_cleanup_artifacts (
    tenant_id INTEGER NOT NULL,
    session_id VARCHAR(64) NOT NULL,
    deletion_revision INTEGER NOT NULL,
    ref VARCHAR(2048) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    state VARCHAR(16) NOT NULL DEFAULT 'pending',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    PRIMARY KEY (tenant_id, session_id, deletion_revision, ref),
    CHECK (state IN ('pending', 'deleted'))
);
CREATE INDEX idx_execution_cleanup_artifacts_ref ON execution_cleanup_artifacts (tenant_id, ref, state);
