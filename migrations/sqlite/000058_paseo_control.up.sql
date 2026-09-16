ALTER TABLE workbench_interactions ADD COLUMN external_pending_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE workbench_interactions ADD COLUMN credential_version INTEGER NOT NULL DEFAULT 0;
CREATE TABLE execution_workspace_leases (
    tenant_id INTEGER NOT NULL,
    workspace_ref VARCHAR(512) NOT NULL,
    run_id VARCHAR(128) NOT NULL,
    owner VARCHAR(512) NOT NULL,
    epoch INTEGER NOT NULL,
    expires_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, workspace_ref),
    CHECK (epoch > 0)
);
CREATE INDEX idx_execution_workspace_leases_expiry ON execution_workspace_leases (expires_at);
