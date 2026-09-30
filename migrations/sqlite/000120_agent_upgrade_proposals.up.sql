-- SQLite twin of versioned migration 000200.
--
-- T31 Agent Upgrade Proposal review table (ticket #61, spec §9). Listing
-- 指向新 Release 时为既有 Adoption 生成可审阅升级建议；接受建议只创建固定
-- 到 to_release_id 的新 Variant 草稿，既有 Variant / 本地 Agent Version /
-- Task 永不因此改变。
--
-- to_release_id 不建 agent_releases 外键：跨 Tenant 引入的 Release 位于
-- tenant_introduced_releases，不在 agent_releases（#60 在 000114/000193 放宽
-- adoption 三条 FK 的同一原因）；租户隔离由每条查询的 tenant_id 绑定保持。
CREATE TABLE agent_upgrade_proposals (
 id VARCHAR(36) NOT NULL, tenant_id INTEGER NOT NULL,
 adoption_id VARCHAR(36) NOT NULL, listing_id VARCHAR(36) NOT NULL,
 from_release_id VARCHAR(36) NOT NULL, to_release_id VARCHAR(36) NOT NULL,
 to_semantic_version VARCHAR(64) NOT NULL DEFAULT '',
 diff_json TEXT NOT NULL DEFAULT '{}', state VARCHAR(32) NOT NULL DEFAULT 'open',
 accepted_variant_id VARCHAR(36), resolved_by VARCHAR(255) NOT NULL DEFAULT '',
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (adoption_id, tenant_id) REFERENCES agent_adoptions(id, tenant_id)
);
CREATE INDEX idx_agent_upgrade_proposals_adoption ON agent_upgrade_proposals(tenant_id, adoption_id, created_at);
CREATE UNIQUE INDEX uq_agent_upgrade_proposals_scope ON agent_upgrade_proposals(tenant_id, adoption_id, to_release_id);
