CREATE TABLE artifact_versions (
    tenant_id BIGINT NOT NULL,
    id VARCHAR(64) NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    session_id VARCHAR(64) NOT NULL,
    digest VARCHAR(64) NOT NULL,
    object_key VARCHAR(1024) NOT NULL,
    mime VARCHAR(255) NOT NULL,
    size BIGINT NOT NULL,
    scan_state VARCHAR(16) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (size > 0),
    CHECK (scan_state IN ('pending', 'uploaded', 'clean', 'quarantined', 'ready'))
);
CREATE UNIQUE INDEX artifact_version_identity ON artifact_versions (tenant_id, id);
CREATE INDEX artifact_version_run ON artifact_versions (tenant_id, run_id);
