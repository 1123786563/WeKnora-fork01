ALTER TABLE workbench_interactions ADD COLUMN external_pending_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE workbench_interactions ADD COLUMN credential_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE workbench_interactions ADD CONSTRAINT ck_workbench_interaction_credential_version CHECK (credential_version >= 0);
CREATE TABLE execution_workspace_leases (
    tenant_id BIGINT NOT NULL,
    workspace_ref VARCHAR(512) NOT NULL,
    run_id VARCHAR(128) NOT NULL,
    owner VARCHAR(512) NOT NULL,
    epoch BIGINT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, workspace_ref),
    CONSTRAINT ck_execution_workspace_lease_epoch CHECK (epoch > 0)
);
CREATE INDEX idx_execution_workspace_leases_expiry ON execution_workspace_leases (expires_at);
