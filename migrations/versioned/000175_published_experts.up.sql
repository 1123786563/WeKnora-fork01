-- Description: Tenant-internal expert market for the M4 skill market (Task 5).
-- One live row per (tenant, agent) a member published to the
-- workspace-internal expert market. Unlike published_skills (a visibility
-- flag over a shared catalog row), this row owns an IMMUTABLE SNAPSHOT: a
-- materialized expert directory (manifest.yaml + persona doc, skills as
-- references) written atomically into tenant storage at publish time. The
-- row carries name/description AT PUBLISH plus the snapshot ref and content
-- digest; installs copy the snapshot into the tenant's installed-experts
-- root, record an expert_installs row on the manifest slug
-- (tenant-expert-<agentID>) and instantiate through the M2 ExpertService.
-- Unpublish soft-deletes the row (the snapshot stays on disk — immutable
-- history); a soft-deleted scope frees the unique slot so a later publish
-- reinserts fresh (the published_skills semantics).
DO $$ BEGIN RAISE NOTICE '[Migration 000175] Creating published_experts'; END $$;

CREATE TABLE IF NOT EXISTS published_experts (
    id              VARCHAR(36)   NOT NULL,
    tenant_id       BIGINT        NOT NULL,
    agent_id        VARCHAR(36)   NOT NULL,
    name            VARCHAR(255)  NOT NULL,
    description     TEXT,
    snapshot_ref    VARCHAR(1024) NOT NULL,
    snapshot_sha256 VARCHAR(64)   NOT NULL,
    published_by    VARCHAR(255)  NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    PRIMARY KEY (id, tenant_id)
);

COMMENT ON COLUMN published_experts.agent_id IS 'custom_agents.id of the source agent the snapshot was exported from';
COMMENT ON COLUMN published_experts.name IS 'expert display name recorded at publish time (body override or agent name; zh/en passthrough lives in the snapshot manifest)';
COMMENT ON COLUMN published_experts.snapshot_ref IS 'absolute path of the immutable materialized-expert snapshot directory under the tenant published-experts root';
COMMENT ON COLUMN published_experts.snapshot_sha256 IS 'content digest of the snapshot tree at write time (experts.MaterializedExpert.SnapshotSHA256)';
COMMENT ON COLUMN published_experts.published_by IS 'Publishing user id (empty means a system/API-key principal)';

-- Soft-delete aware: a deleted (tenant, agent) row frees the slot so a later
-- publish reinserts in place, matching uq_published_skills_scope.
CREATE UNIQUE INDEX IF NOT EXISTS uq_published_experts_scope
    ON published_experts (tenant_id, agent_id) WHERE deleted_at IS NULL;
