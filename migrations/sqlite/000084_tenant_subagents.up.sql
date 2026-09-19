-- M3: SQLite twin of 000163. One row per (slug, locale) of the builtin
-- sub-agent role library, synced from config/subagents/library. Content is
-- the role markdown verbatim, frontmatter fence included.

CREATE TABLE IF NOT EXISTS tenant_subagents (
    id         VARCHAR(36) NOT NULL,
    tenant_id  INTEGER NOT NULL,
    slug       VARCHAR(255) NOT NULL,
    locale     VARCHAR(8) NOT NULL,
    content    TEXT NOT NULL,
    division   VARCHAR(64) NOT NULL DEFAULT '',
    source     VARCHAR(32) NOT NULL DEFAULT 'builtin',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    PRIMARY KEY (id, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_tenant_subagents_tenant
    ON tenant_subagents(tenant_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_subagents_scope
    ON tenant_subagents(tenant_id, slug, locale) WHERE deleted_at IS NULL;
