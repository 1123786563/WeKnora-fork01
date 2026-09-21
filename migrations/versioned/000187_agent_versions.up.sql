-- Description: Agent-domain immutable Tenant Agent Version snapshots (T28 Wave 1).
-- Freezing an agent APPENDS one row per frozen state: the canonical JSON
-- serialization of the tenant-scoped CustomAgent plus its SHA-256 source
-- digest, with a 1-based version_number allocated inside the scope
-- (tenant_id, agent_id). The row is the fixed source the tenant Marketplace
-- later references by ID and digest; nothing ever updates or deletes it —
-- no updated_at column (nothing may change), no deleted_at (no soft-delete
-- resurrection), and the repository layer offers append/read operations
-- only. snapshot is deliberately TEXT, not JSONB: JSONB normalizes key
-- order/whitespace, which would break the digest-verified byte-exact
-- round-trip on read.
DO $$ BEGIN RAISE NOTICE '[Migration 000178] Creating agent_versions'; END $$;

CREATE TABLE IF NOT EXISTS agent_versions (
    id             VARCHAR(36)  NOT NULL,
    tenant_id      BIGINT       NOT NULL,
    agent_id       VARCHAR(36)  NOT NULL,
    version_number INTEGER      NOT NULL CHECK (version_number >= 1),
    snapshot       TEXT         NOT NULL,
    source_sha256  VARCHAR(64)  NOT NULL,
    frozen_by      VARCHAR(255) NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, tenant_id)
);

COMMENT ON TABLE agent_versions IS 'append-only immutable frozen snapshots of tenant custom agents; the Agent domain owns this table, the Marketplace only references (id, source_sha256)';
COMMENT ON COLUMN agent_versions.agent_id IS 'custom_agents.id of the source agent at freeze time';
COMMENT ON COLUMN agent_versions.version_number IS '1-based monotonic sequence within (tenant_id, agent_id); the unique index is the hard no-collision guard';
COMMENT ON COLUMN agent_versions.snapshot IS 'canonical JSON of the frozen CustomAgent; TEXT (not JSONB) so the bytes read back are the exact bytes hashed at freeze time';
COMMENT ON COLUMN agent_versions.source_sha256 IS 'SHA-256 hex digest over the snapshot bytes at freeze time; reads verify against it';
COMMENT ON COLUMN agent_versions.frozen_by IS 'freezing user id (empty means a system/API-key principal)';

CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_versions_scope
    ON agent_versions (tenant_id, agent_id, version_number);
