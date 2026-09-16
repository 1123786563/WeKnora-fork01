-- W33: durable session deletion barriers and fenced cleanup claims.
-- No FK to sessions: the tombstone must outlive the deleted session.
CREATE TABLE execution_cleanup (
    tenant_id INTEGER NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    session_id VARCHAR(64) NOT NULL,
    deletion_revision INTEGER NOT NULL DEFAULT 1,
    state VARCHAR(32) NOT NULL DEFAULT 'tombstoned',
    worker VARCHAR(128) NOT NULL DEFAULT '',
    epoch INTEGER NOT NULL DEFAULT 0,
    lease_until DATETIME,
    stopped BOOLEAN NOT NULL DEFAULT 0,
    settled BOOLEAN NOT NULL DEFAULT 0,
    retention_elapsed BOOLEAN NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, session_id),
    CHECK (state IN ('tombstoned', 'cleanup_claimed', 'cleanup_pending', 'purged'))
);
CREATE INDEX idx_execution_cleanup_claim ON execution_cleanup (state, lease_until, updated_at);
CREATE INDEX idx_execution_cleanup_owner ON execution_cleanup (tenant_id, owner_id, session_id);
