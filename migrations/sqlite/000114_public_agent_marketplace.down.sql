-- Down twin: restore the #59 FKs by rebuilding both tables with their
-- original 000113 DDL, then drop the public marketplace tables. Like the up
-- twin this file owns its transaction and requires the driver's NoTxWrap
-- (PRAGMA foreign_keys only changes outside a transaction). Restoring the
-- FKs fails on live data when platform-lineage Adoption rows exist — the
-- same expectation documented on the versioned twin.
PRAGMA foreign_keys = OFF;
BEGIN IMMEDIATE;
DROP TABLE IF EXISTS tenant_introduced_releases;
DROP TABLE IF EXISTS public_agent_releases;
DROP TABLE IF EXISTS public_release_reviews;
DROP TABLE IF EXISTS public_release_submissions;
DROP TABLE IF EXISTS public_marketplace_listings;
DROP TABLE IF EXISTS public_marketplace_verified_publishers;
CREATE TABLE agent_adoptions_restore (
 id VARCHAR(36) NOT NULL, tenant_id INTEGER NOT NULL, listing_id VARCHAR(36) NOT NULL,
 accepted_release_id VARCHAR(36) NOT NULL, state VARCHAR(32) NOT NULL DEFAULT 'active',
 created_by VARCHAR(255) NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (listing_id, tenant_id) REFERENCES agent_marketplace_listings(id, tenant_id),
 FOREIGN KEY (accepted_release_id, tenant_id) REFERENCES agent_releases(id, tenant_id)
);
INSERT INTO agent_adoptions_restore (id, tenant_id, listing_id, accepted_release_id, state, created_by, created_at, updated_at)
 SELECT id, tenant_id, listing_id, accepted_release_id, state, created_by, created_at, updated_at FROM agent_adoptions;
DROP TABLE agent_adoptions;
ALTER TABLE agent_adoptions_restore RENAME TO agent_adoptions;
CREATE UNIQUE INDEX uq_agent_adoptions_scope ON agent_adoptions(tenant_id, listing_id);
CREATE TABLE agent_adoption_variants_restore (
 id VARCHAR(36) NOT NULL, tenant_id INTEGER NOT NULL, adoption_id VARCHAR(36) NOT NULL, release_id VARCHAR(36) NOT NULL,
 name VARCHAR(255) NOT NULL, state VARCHAR(32) NOT NULL DEFAULT 'draft',
 local_agent_id VARCHAR(36), local_agent_version_id VARCHAR(36),
 created_by VARCHAR(255) NOT NULL DEFAULT '', tested_by VARCHAR(255) NOT NULL DEFAULT '', tested_at DATETIME,
 published_by VARCHAR(255) NOT NULL DEFAULT '', published_at DATETIME,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (adoption_id, tenant_id) REFERENCES agent_adoptions(id, tenant_id),
 FOREIGN KEY (release_id, tenant_id) REFERENCES agent_releases(id, tenant_id)
);
INSERT INTO agent_adoption_variants_restore (id, tenant_id, adoption_id, release_id, name, state, local_agent_id, local_agent_version_id, created_by, tested_by, tested_at, published_by, published_at, created_at, updated_at)
 SELECT id, tenant_id, adoption_id, release_id, name, state, local_agent_id, local_agent_version_id, created_by, tested_by, tested_at, published_by, published_at, created_at, updated_at FROM agent_adoption_variants;
DROP TABLE agent_adoption_variants;
ALTER TABLE agent_adoption_variants_restore RENAME TO agent_adoption_variants;
CREATE INDEX idx_agent_adoption_variants_adoption ON agent_adoption_variants(tenant_id, adoption_id, created_at);
COMMIT;
PRAGMA foreign_keys = ON;
