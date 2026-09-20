-- Description: Per-tenant expert install ledger for the M4 skill market.
-- One row per (tenant, skillset slug) installed from the SkillHub market.
-- StorageRef names the materialized expert tree (ScanExperts layout) under
-- the tenant-scoped expert-market root; SnapshotSHA256 is the content digest
-- of that tree at write time, so reinstalling unchanged content is
-- recognizable without re-reading storage. The catalog reads the trees, not
-- these rows; the ledger is the idempotency and audit record.
DO $$ BEGIN RAISE NOTICE '[Migration 000173] Creating expert_installs'; END $$;

CREATE TABLE IF NOT EXISTS expert_installs (
    id              VARCHAR(36)   NOT NULL,
    tenant_id       BIGINT        NOT NULL,
    slug            VARCHAR(255)  NOT NULL,
    storage_ref     VARCHAR(1024) NOT NULL,
    snapshot_sha256 VARCHAR(64)   NOT NULL,
    created_by      VARCHAR(255)  NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    PRIMARY KEY (id, tenant_id)
);

COMMENT ON COLUMN expert_installs.slug IS 'SkillHub skillset slug; the materialized expert ID is skillhub-skillset-<slug>';
COMMENT ON COLUMN expert_installs.storage_ref IS 'Tenant-scoped directory of the materialized expert tree (manifest.yaml + SOUL.md + skills/<slug>/)';
COMMENT ON COLUMN expert_installs.snapshot_sha256 IS 'SHA-256 over the sorted (path, bytes) pairs of the materialized tree at write time';
COMMENT ON COLUMN expert_installs.created_by IS 'Installing user (id or name); empty means a system-triggered install';

-- Soft-delete aware: a deleted (tenant, slug) row frees the slot so a later
-- reinstall reinserts in place, matching uq_tenant_subagents_scope.
CREATE UNIQUE INDEX IF NOT EXISTS uq_expert_installs_scope
    ON expert_installs (tenant_id, slug) WHERE deleted_at IS NULL;
