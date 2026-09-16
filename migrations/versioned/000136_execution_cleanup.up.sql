-- W33: durable session deletion barriers and fenced cleanup claims.
-- This ledger intentionally has no FK to sessions: the tombstone must outlive
-- the soft/deleted session and protect delayed remote observations.
CREATE TABLE execution_cleanup (
    tenant_id BIGINT NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    session_id VARCHAR(64) NOT NULL,
    deletion_revision BIGINT NOT NULL DEFAULT 1,
    state VARCHAR(32) NOT NULL DEFAULT 'tombstoned',
    worker VARCHAR(128) NOT NULL DEFAULT '',
    epoch BIGINT NOT NULL DEFAULT 0,
    lease_until TIMESTAMPTZ,
    stopped BOOLEAN NOT NULL DEFAULT FALSE,
    settled BOOLEAN NOT NULL DEFAULT FALSE,
    retention_elapsed BOOLEAN NOT NULL DEFAULT FALSE,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, session_id),
    CHECK (state IN ('tombstoned', 'cleanup_claimed', 'cleanup_pending', 'purged'))
);
CREATE INDEX idx_execution_cleanup_claim ON execution_cleanup (state, lease_until, updated_at);
CREATE INDEX idx_execution_cleanup_owner ON execution_cleanup (tenant_id, owner_id, session_id);
