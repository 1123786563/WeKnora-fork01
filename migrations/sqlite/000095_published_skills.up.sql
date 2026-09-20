-- M4: SQLite twin of 000174. One live row per (tenant, catalog skill)
-- published to the workspace-internal skill market. The row is a visibility
-- flag on the shared tenant_skill_catalog definition, not a content copy:
-- publisher and installer are members of the SAME tenant, so install reuses
-- the catalog row's own bundle via InstallCatalogToConfigs.

CREATE TABLE IF NOT EXISTS published_skills (
    id           VARCHAR(36) NOT NULL,
    tenant_id    INTEGER NOT NULL,
    catalog_id   VARCHAR(36) NOT NULL,
    published_by VARCHAR(255) NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at   DATETIME,
    PRIMARY KEY (id, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_published_skills_tenant
    ON published_skills(tenant_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_published_skills_scope
    ON published_skills (tenant_id, catalog_id) WHERE deleted_at IS NULL;
