-- One-time proof challenges and owner-scoped personal node registrations.
-- No bearer, private key, or node root is persisted here.
CREATE TABLE execution_registration_challenges (
    tenant_id BIGINT NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    challenge_id VARCHAR(128) NOT NULL,
    runtime_id VARCHAR(255) NOT NULL,
    external_target_id VARCHAR(512) NOT NULL,
    public_key_fingerprint VARCHAR(64) NOT NULL,
    nonce_hash VARCHAR(64) NOT NULL,
    nonce VARCHAR(512) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id, challenge_id)
);
CREATE INDEX idx_execution_registration_challenges_expiry ON execution_registration_challenges (expires_at, consumed_at);

CREATE TABLE execution_registrations (
    tenant_id BIGINT NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    registration_id VARCHAR(128) NOT NULL,
    runtime_id VARCHAR(255) NOT NULL,
    external_target_id VARCHAR(512) NOT NULL,
    public_key_fingerprint VARCHAR(64) NOT NULL,
    credential_version BIGINT NOT NULL DEFAULT 1,
    state VARCHAR(32) NOT NULL DEFAULT 'active',
    idempotency_key VARCHAR(128) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMPTZ,
    PRIMARY KEY (tenant_id, owner_id, registration_id),
    CONSTRAINT ck_execution_registrations_version CHECK (credential_version > 0),
    CONSTRAINT ck_execution_registrations_state CHECK (state IN ('active', 'revoked'))
);
CREATE UNIQUE INDEX uq_execution_registrations_idempotency ON execution_registrations (tenant_id, owner_id, idempotency_key);
CREATE UNIQUE INDEX uq_execution_registrations_target_active ON execution_registrations (tenant_id, runtime_id, external_target_id) WHERE state = 'active';
CREATE INDEX idx_execution_registrations_owner ON execution_registrations (tenant_id, owner_id, state, created_at);
