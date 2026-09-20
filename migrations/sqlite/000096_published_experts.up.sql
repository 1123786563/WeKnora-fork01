-- M4: SQLite twin of 000175. One live row per (tenant, agent) published to
-- the workspace-internal expert market. The row owns an immutable
-- materialized-expert snapshot (manifest.yaml + persona doc, skills as
-- references) in tenant storage; installs copy the snapshot into the
-- tenant's installed-experts root, record an expert_installs row on the
-- manifest slug (tenant-expert-<agentID>) and instantiate via the M2
-- ExpertService. Unpublish soft-deletes the row.

CREATE TABLE IF NOT EXISTS published_experts (
    id              VARCHAR(36)   NOT NULL,
    tenant_id       INTEGER       NOT NULL,
    agent_id        VARCHAR(36)   NOT NULL,
    name            VARCHAR(255)  NOT NULL,
    description     TEXT,
    snapshot_ref    VARCHAR(1024) NOT NULL,
    snapshot_sha256 VARCHAR(64)   NOT NULL,
    published_by    VARCHAR(255)  NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at      DATETIME,
    PRIMARY KEY (id, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_published_experts_tenant
    ON published_experts(tenant_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_published_experts_scope
    ON published_experts (tenant_id, agent_id) WHERE deleted_at IS NULL;
