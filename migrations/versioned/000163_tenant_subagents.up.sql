-- Description: Builtin sub-agent role library rows synced from
-- config/subagents/library. One row is one (slug, locale) pair: Content holds
-- the role markdown verbatim, frontmatter fence included, so a sync can
-- rewrite what the catalog serves without re-parsing the library at read time.
DO $$ BEGIN RAISE NOTICE '[Migration 000163] Creating tenant_subagents'; END $$;

CREATE TABLE IF NOT EXISTS tenant_subagents (
    id         VARCHAR(36)  NOT NULL,
    tenant_id  BIGINT       NOT NULL,
    slug       VARCHAR(255) NOT NULL,
    locale     VARCHAR(8)   NOT NULL,
    content    TEXT         NOT NULL,
    division   VARCHAR(64)  NOT NULL DEFAULT '',
    source     VARCHAR(32)  NOT NULL DEFAULT 'builtin',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    PRIMARY KEY (id, tenant_id)
);

COMMENT ON COLUMN tenant_subagents.content IS 'Full role markdown verbatim, frontmatter included; the body becomes the sub-run system prompt';
COMMENT ON COLUMN tenant_subagents.division IS 'Division directory name under config/subagents/library/<locale>';
COMMENT ON COLUMN tenant_subagents.source IS 'Where the role came from; "builtin" for library-shipped roles';

-- Soft-delete aware: a deleted (tenant, slug, locale) row frees the slot so a
-- later sync can reinsert in place, matching uq_tenant_skills_config_name.
CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_subagents_scope
    ON tenant_subagents (tenant_id, slug, locale) WHERE deleted_at IS NULL;
