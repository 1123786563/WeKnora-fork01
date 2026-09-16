CREATE TABLE execution_targets (
    tenant_id INTEGER NOT NULL,
    id VARCHAR(128) NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    kind VARCHAR(64) NOT NULL,
    state VARCHAR(32) NOT NULL DEFAULT 'active',
    credential_version INTEGER NOT NULL,
    runtime_id VARCHAR(255) NOT NULL,
    external_target_id VARCHAR(512) NOT NULL,
    root_ref VARCHAR(1024) NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at DATETIME,
    PRIMARY KEY (tenant_id, id),
    CHECK (state IN ('active', 'revoked')),
    CHECK (credential_version > 0),
    UNIQUE (runtime_id, external_target_id)
);
CREATE INDEX idx_execution_targets_owner
    ON execution_targets (tenant_id, owner_id, state, created_at);
CREATE TABLE execution_workspaces (
    tenant_id INTEGER NOT NULL,
    id VARCHAR(128) NOT NULL,
    target_id VARCHAR(128) NOT NULL,
    root_ref VARCHAR(1024) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id, target_id)
        REFERENCES execution_targets (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_execution_workspaces_target
    ON execution_workspaces (tenant_id, target_id);
