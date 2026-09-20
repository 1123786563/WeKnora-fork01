-- T28: SQLite twin of 000178. Append-only immutable frozen snapshots of
-- tenant custom agents, owned by the Agent domain. One row per freeze: the
-- canonical JSON of the tenant-scoped CustomAgent (stored as TEXT so the
-- bytes read back are the exact bytes hashed at freeze time) plus its
-- SHA-256 source digest, with a 1-based version_number unique within
-- (tenant_id, agent_id). No updated_at / deleted_at: nothing may update or
-- delete a frozen version.

CREATE TABLE IF NOT EXISTS agent_versions (
    id             VARCHAR(36)  NOT NULL,
    tenant_id      INTEGER      NOT NULL,
    agent_id       VARCHAR(36)  NOT NULL,
    version_number INTEGER      NOT NULL CHECK (version_number >= 1),
    snapshot       TEXT         NOT NULL,
    source_sha256  VARCHAR(64)  NOT NULL,
    frozen_by      VARCHAR(255) NOT NULL DEFAULT '',
    created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, tenant_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_versions_scope
    ON agent_versions (tenant_id, agent_id, version_number);
