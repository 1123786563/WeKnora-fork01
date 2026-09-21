-- Native OAuth authority stores digests only, never state/proof/token material.
CREATE TABLE IF NOT EXISTS native_agent_oauth_attempts (
    tenant_id INTEGER NOT NULL,
    run_id TEXT NOT NULL,
    pending_id TEXT NOT NULL,
    attempt_id TEXT NOT NULL CHECK (length(attempt_id) > 0),
    session_id TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    principal_type TEXT NOT NULL,
    principal_id TEXT NOT NULL,
    service_id TEXT NOT NULL,
    installation_id TEXT NOT NULL DEFAULT '',
    pending_revision INTEGER NOT NULL CHECK (pending_revision > 0),
    redirect_uri TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    binding_hash TEXT NOT NULL CHECK (length(binding_hash) = 64),
    state_hash TEXT NOT NULL CHECK (length(state_hash) = 64),
    receipt_hash TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1 CHECK (revision IN (1, 2)),
    consumed_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, attempt_id),
    UNIQUE (tenant_id, state_hash),
    FOREIGN KEY (tenant_id, run_id, pending_id) REFERENCES native_agent_pending_decisions(tenant_id, run_id, pending_id) ON DELETE RESTRICT,
    CHECK ((revision = 1 AND consumed_at IS NULL AND receipt_hash = '') OR
           (revision = 2 AND consumed_at IS NOT NULL AND length(receipt_hash) = 64))
);
CREATE INDEX IF NOT EXISTS idx_native_agent_oauth_pending
    ON native_agent_oauth_attempts (tenant_id, run_id, pending_id, revision, expires_at);
