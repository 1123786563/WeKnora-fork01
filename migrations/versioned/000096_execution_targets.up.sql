-- Server-owned execution targets and opaque workspace bindings.
CREATE TABLE execution_targets (
    tenant_id BIGINT NOT NULL,
    id VARCHAR(128) NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    kind VARCHAR(64) NOT NULL,
    state VARCHAR(32) NOT NULL DEFAULT 'active',
    credential_version BIGINT NOT NULL,
    runtime_id VARCHAR(255) NOT NULL,
    external_target_id VARCHAR(512) NOT NULL,
    root_ref VARCHAR(1024) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMPTZ,
    PRIMARY KEY (tenant_id, id),
    CONSTRAINT ck_execution_targets_state CHECK (state IN ('active', 'revoked')),
    CONSTRAINT ck_execution_targets_credential_version CHECK (credential_version > 0)
);
CREATE UNIQUE INDEX uq_execution_targets_runtime_external
    ON execution_targets (runtime_id, external_target_id);
CREATE INDEX idx_execution_targets_owner
    ON execution_targets (tenant_id, owner_id, state, created_at);

CREATE TABLE execution_target_identities (
    tenant_id BIGINT NOT NULL,
    runtime_id VARCHAR(255) NOT NULL,
    external_target_id VARCHAR(512) NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    credential_version BIGINT NOT NULL,
    state VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, runtime_id, external_target_id),
    CONSTRAINT ck_execution_target_identities_version CHECK (credential_version > 0),
    CONSTRAINT ck_execution_target_identities_state CHECK (state IN ('active', 'revoked'))
);
CREATE INDEX idx_execution_target_identities_owner
    ON execution_target_identities (tenant_id, owner_id, state);

CREATE TABLE execution_workspaces (
    tenant_id BIGINT NOT NULL,
    id VARCHAR(128) NOT NULL,
    target_id VARCHAR(128) NOT NULL,
    root_ref VARCHAR(1024) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, id),
    CONSTRAINT fk_execution_workspaces_target
        FOREIGN KEY (tenant_id, target_id)
        REFERENCES execution_targets (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_execution_workspaces_target
    ON execution_workspaces (tenant_id, target_id);
