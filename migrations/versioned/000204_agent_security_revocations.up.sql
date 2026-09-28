-- T34 (#64) security revocation ledgers (spec §10/§11, CONTEXT.md「安全撤回」
-- 「依赖安全阻断」). Append-only history: re-revocation adds a NEW row (the
-- latest per target is authoritative); nothing here deletes releases,
-- variants, tasks or audit rows. release_id / 替代 release 无 agent_releases
-- 外键（引入式 Release 位于 tenant_introduced_releases，#60 在 000114/000193
-- 放宽 adoption FK 的同一原因）；租户隔离由每条查询的 tenant_id 绑定保持。
CREATE TABLE agent_release_revocations (
 id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL,
 listing_id VARCHAR(36) NOT NULL DEFAULT '', release_id VARCHAR(36) NOT NULL,
 reason TEXT NOT NULL,
 replacement_release_id VARCHAR(36) NOT NULL DEFAULT '',
 in_flight_disposition VARCHAR(16) NOT NULL DEFAULT 'cancel',
 canceled_run_count BIGINT NOT NULL DEFAULT 0,
 revoked_by VARCHAR(255) NOT NULL DEFAULT '',
 revoked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (id, tenant_id)
);
CREATE INDEX idx_agent_release_revocations_release ON agent_release_revocations(tenant_id, release_id, created_at);
CREATE TABLE agent_dependency_revocations (
 id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL,
 dep_type VARCHAR(32) NOT NULL, dep_id VARCHAR(255) NOT NULL,
 dep_version VARCHAR(64) NOT NULL, dep_digest VARCHAR(64) NOT NULL,
 reason TEXT NOT NULL,
 replacement_version VARCHAR(64) NOT NULL DEFAULT '',
 in_flight_disposition VARCHAR(16) NOT NULL DEFAULT 'cancel',
 canceled_run_count BIGINT NOT NULL DEFAULT 0,
 revoked_by VARCHAR(255) NOT NULL DEFAULT '',
 revoked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (id, tenant_id)
);
CREATE INDEX idx_agent_dependency_revocations_identity ON agent_dependency_revocations(tenant_id, dep_type, dep_id, dep_version, dep_digest, created_at);
