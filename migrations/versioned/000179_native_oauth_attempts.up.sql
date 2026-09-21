-- Native OAuth authority stores digests only, never state/proof/token material.
CREATE TABLE IF NOT EXISTS native_agent_oauth_attempts (
    tenant_id BIGINT NOT NULL,
    run_id VARCHAR(255) NOT NULL,
    pending_id VARCHAR(255) NOT NULL,
    attempt_id VARCHAR(255) NOT NULL CHECK (length(attempt_id) > 0),
    session_id VARCHAR(255) NOT NULL,
    owner_id VARCHAR(255) NOT NULL,
    principal_type VARCHAR(64) NOT NULL,
    principal_id VARCHAR(255) NOT NULL,
    service_id VARCHAR(255) NOT NULL,
    installation_id VARCHAR(255) NOT NULL DEFAULT '',
    pending_revision BIGINT NOT NULL CHECK (pending_revision > 0),
    redirect_uri TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    binding_hash VARCHAR(64) NOT NULL CHECK (length(binding_hash) = 64),
    state_hash VARCHAR(64) NOT NULL CHECK (length(state_hash) = 64),
    receipt_hash VARCHAR(64) NOT NULL DEFAULT '',
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision IN (1, 2)),
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, attempt_id),
    UNIQUE (tenant_id, state_hash),
    FOREIGN KEY (tenant_id, run_id, pending_id) REFERENCES native_agent_pending_decisions(tenant_id, run_id, pending_id) ON DELETE RESTRICT,
    CHECK ((revision = 1 AND consumed_at IS NULL AND receipt_hash = '') OR
           (revision = 2 AND consumed_at IS NOT NULL AND length(receipt_hash) = 64))
);
CREATE INDEX IF NOT EXISTS idx_native_agent_oauth_pending
    ON native_agent_oauth_attempts (tenant_id, run_id, pending_id, revision, expires_at);
