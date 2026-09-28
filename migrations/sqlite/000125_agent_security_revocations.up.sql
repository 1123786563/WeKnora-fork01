-- SQLite twin of versioned migration 000204.
-- T34 (#64) append-only security revocation ledgers. Release rows deliberately
-- have no agent_releases FK because introduced releases live in
-- tenant_introduced_releases; tenant isolation is enforced by tenant-scoped queries.
CREATE TABLE agent_release_revocations (
 id VARCHAR(36) NOT NULL, tenant_id INTEGER NOT NULL,
 listing_id VARCHAR(36) NOT NULL DEFAULT '', release_id VARCHAR(36) NOT NULL,
 reason TEXT NOT NULL,
 replacement_release_id VARCHAR(36) NOT NULL DEFAULT '',
 in_flight_disposition VARCHAR(16) NOT NULL DEFAULT 'cancel',
 canceled_run_count INTEGER NOT NULL DEFAULT 0,
 revoked_by VARCHAR(255) NOT NULL DEFAULT '',
 revoked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (id, tenant_id)
);
CREATE INDEX idx_agent_release_revocations_release ON agent_release_revocations(tenant_id, release_id, created_at);
CREATE TABLE agent_dependency_revocations (
 id VARCHAR(36) NOT NULL, tenant_id INTEGER NOT NULL,
 dep_type VARCHAR(32) NOT NULL, dep_id VARCHAR(255) NOT NULL,
 dep_version VARCHAR(64) NOT NULL, dep_digest VARCHAR(64) NOT NULL,
 reason TEXT NOT NULL,
 replacement_version VARCHAR(64) NOT NULL DEFAULT '',
 in_flight_disposition VARCHAR(16) NOT NULL DEFAULT 'cancel',
 canceled_run_count INTEGER NOT NULL DEFAULT 0,
 revoked_by VARCHAR(255) NOT NULL DEFAULT '',
 revoked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (id, tenant_id)
);
CREATE INDEX idx_agent_dependency_revocations_identity ON agent_dependency_revocations(tenant_id, dep_type, dep_id, dep_version, dep_digest, created_at);
