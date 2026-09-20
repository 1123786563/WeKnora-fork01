-- Description: Tenant-internal skill market for the M4 skill market (Task 4).
-- One live row per (tenant, catalog skill) an admin published to the
-- workspace-internal market. The row is a visibility FLAG on the existing
-- tenant_skill_catalog definition, not a content copy: publisher and
-- installer are members of the SAME tenant and share the catalog row, its
-- bundle object and the sandbox configs, so install reuses
-- InstallCatalogToConfigs on the shared definition and never re-registers
-- content. Unpublish soft-deletes; a soft-deleted scope frees the unique
-- slot so a later publish reinserts fresh (the expert_installs semantics).
DO $$ BEGIN RAISE NOTICE '[Migration 000175] Creating published_skills'; END $$;

CREATE TABLE IF NOT EXISTS published_skills (
    id           VARCHAR(36)   NOT NULL,
    tenant_id    BIGINT        NOT NULL,
    catalog_id   VARCHAR(36)   NOT NULL,
    published_by VARCHAR(255)  NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ,
    PRIMARY KEY (id, tenant_id)
);

COMMENT ON COLUMN published_skills.catalog_id IS 'tenant_skill_catalog.id of the shared workspace definition this row flags as market-visible';
COMMENT ON COLUMN published_skills.published_by IS 'Publishing user id (empty means a system/API-key principal)';

-- Soft-delete aware: a deleted (tenant, catalog) row frees the slot so a
-- later publish reinserts in place, matching uq_expert_installs_scope.
CREATE UNIQUE INDEX IF NOT EXISTS uq_published_skills_scope
    ON published_skills (tenant_id, catalog_id) WHERE deleted_at IS NULL;
