-- M4: SQLite twin of 000174. One row per (tenant, skillset slug) installed
-- from the SkillHub market. StorageRef names the materialized expert tree
-- (ScanExperts layout) under the tenant-scoped expert-market root;
-- SnapshotSHA256 is the content digest of that tree at write time.

CREATE TABLE IF NOT EXISTS expert_installs (
    id              VARCHAR(36) NOT NULL,
    tenant_id       INTEGER NOT NULL,
    slug            VARCHAR(255) NOT NULL,
    storage_ref     VARCHAR(1024) NOT NULL,
    snapshot_sha256 VARCHAR(64) NOT NULL,
    created_by      VARCHAR(255) NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at      DATETIME,
    PRIMARY KEY (id, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_expert_installs_tenant
    ON expert_installs(tenant_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_expert_installs_scope
    ON expert_installs (tenant_id, slug) WHERE deleted_at IS NULL;
