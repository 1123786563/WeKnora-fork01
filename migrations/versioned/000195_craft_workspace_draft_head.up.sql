CREATE UNIQUE INDEX uq_craft_workspaces_tenant_id ON craft_workspaces (tenant_id, id);

CREATE TABLE craft_workspace_draft_heads (
    workspace_id VARCHAR(64) PRIMARY KEY NOT NULL,
    tenant_id BIGINT NOT NULL,
    revision BIGINT NOT NULL CHECK (revision >= 0),
    state VARCHAR(16) NOT NULL CHECK (state IN ('empty', 'selected')),
    source_run_id VARCHAR(64) NULL,
    manifest_digest VARCHAR(64) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ((state = 'empty' AND revision = 0 AND source_run_id IS NULL AND manifest_digest IS NULL)
        OR (state = 'selected' AND revision > 0 AND source_run_id IS NOT NULL AND manifest_digest IS NOT NULL)),
    UNIQUE (tenant_id, workspace_id),
    FOREIGN KEY (tenant_id, workspace_id) REFERENCES craft_workspaces(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE craft_workspace_draft_revisions (
    workspace_id VARCHAR(64) NOT NULL,
    revision BIGINT NOT NULL CHECK (revision > 0),
    tenant_id BIGINT NOT NULL,
    source_run_id VARCHAR(64) NOT NULL,
    manifest_digest VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (workspace_id, revision),
    FOREIGN KEY (tenant_id, workspace_id) REFERENCES craft_workspace_draft_heads(tenant_id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, source_run_id) REFERENCES agent_runs(tenant_id, run_id) ON DELETE RESTRICT
);

CREATE TABLE craft_workspace_draft_files (
    workspace_id VARCHAR(64) NOT NULL,
    revision BIGINT NOT NULL,
    path VARCHAR(512) NOT NULL,
    object_ref TEXT NOT NULL,
    sha256 VARCHAR(64) NOT NULL,
    bytes BIGINT NOT NULL CHECK (bytes >= 0),
    mime VARCHAR(255) NOT NULL DEFAULT '',
    PRIMARY KEY (workspace_id, revision, path),
    FOREIGN KEY (workspace_id, revision) REFERENCES craft_workspace_draft_revisions(workspace_id, revision) ON DELETE CASCADE
);
