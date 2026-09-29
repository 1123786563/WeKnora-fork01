-- SQLite twin of versioned migration 000193.
--
-- This migration owns its transaction and MUST run with the driver's
-- NoTxWrap: PRAGMA foreign_keys only changes outside a transaction, and the
-- agent_adoptions/agent_adoption_variants rebuild below (DROP TABLE on a
-- referenced parent) would raise FK violations on live data if the PRAGMAs
-- were swallowed by a per-file transaction wrapper. The production entry
-- internal/database/migration.go runs exactly this file through the
-- dedicated NoTxWrap three-phase block added alongside it (mirroring the
-- 000055 workbench rebuild precedent); test harnesses that hand the whole
-- stream a NoTxWrap driver are equally safe.
PRAGMA foreign_keys = OFF;
BEGIN IMMEDIATE;

CREATE TABLE public_marketplace_verified_publishers (
 tenant_id INTEGER PRIMARY KEY,
 state VARCHAR(32) NOT NULL DEFAULT 'verified',
 verified_by VARCHAR(255) NOT NULL DEFAULT '',
 note TEXT NOT NULL DEFAULT '',
 verified_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE public_marketplace_listings (
 id VARCHAR(36) PRIMARY KEY,
 publisher_tenant_id INTEGER NOT NULL,
 source_listing_id VARCHAR(36) NOT NULL,
 display_name VARCHAR(255) NOT NULL,
 summary TEXT NOT NULL DEFAULT '',
 state VARCHAR(32) NOT NULL DEFAULT 'listed',
 current_release_id VARCHAR(36),
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE (publisher_tenant_id, source_listing_id)
);
CREATE TABLE public_release_submissions (
 id VARCHAR(36) PRIMARY KEY,
 publisher_tenant_id INTEGER NOT NULL,
 public_listing_id VARCHAR(36) NOT NULL,
 source_listing_id VARCHAR(36) NOT NULL,
 source_release_id VARCHAR(36) NOT NULL,
 publisher_actor_id VARCHAR(255) NOT NULL DEFAULT '',
 semantic_version VARCHAR(64) NOT NULL,
 bundle_digest VARCHAR(64) NOT NULL,
 manifest_json TEXT NOT NULL,
 dependency_lock_json TEXT NOT NULL,
 bundle BLOB NOT NULL,
 status VARCHAR(32) NOT NULL DEFAULT 'submitted',
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 FOREIGN KEY (public_listing_id) REFERENCES public_marketplace_listings(id),
 FOREIGN KEY (publisher_tenant_id, source_release_id) REFERENCES agent_releases(tenant_id, id)
);
CREATE TABLE public_release_reviews (
 id VARCHAR(36) PRIMARY KEY,
 submission_id VARCHAR(36) NOT NULL,
 reviewer_id VARCHAR(255) NOT NULL,
 reviewed_digest VARCHAR(64) NOT NULL,
 decision VARCHAR(32) NOT NULL,
 reason TEXT NOT NULL DEFAULT '',
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 FOREIGN KEY (submission_id) REFERENCES public_release_submissions(id)
);
CREATE TABLE public_agent_releases (
 id VARCHAR(36) PRIMARY KEY,
 listing_id VARCHAR(36) NOT NULL,
 submission_id VARCHAR(36) NOT NULL,
 publisher_tenant_id INTEGER NOT NULL,
 release_number INTEGER NOT NULL CHECK (release_number >= 1),
 semantic_version VARCHAR(64) NOT NULL,
 bundle_digest VARCHAR(64) NOT NULL,
 manifest_json TEXT NOT NULL,
 dependency_lock_json TEXT NOT NULL,
 bundle BLOB NOT NULL,
 published_by VARCHAR(255) NOT NULL DEFAULT '',
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 FOREIGN KEY (listing_id) REFERENCES public_marketplace_listings(id),
 FOREIGN KEY (submission_id) REFERENCES public_release_submissions(id)
);
CREATE INDEX idx_public_release_submissions_queue ON public_release_submissions(status, created_at);
CREATE INDEX idx_public_release_reviews_submission ON public_release_reviews(submission_id, created_at);
CREATE INDEX idx_public_agent_releases_listing ON public_agent_releases(listing_id, release_number);
CREATE UNIQUE INDEX uq_public_release_review_submission ON public_release_reviews(submission_id);
CREATE UNIQUE INDEX uq_public_agent_releases_number ON public_agent_releases(listing_id, release_number);
CREATE UNIQUE INDEX uq_public_agent_releases_semantic ON public_agent_releases(listing_id, semantic_version);
CREATE UNIQUE INDEX uq_public_agent_releases_digest ON public_agent_releases(listing_id, bundle_digest);
CREATE TABLE tenant_introduced_releases (
 id VARCHAR(36) NOT NULL,
 tenant_id INTEGER NOT NULL,
 public_listing_id VARCHAR(36) NOT NULL,
 public_release_id VARCHAR(36) NOT NULL,
 display_name VARCHAR(255) NOT NULL,
 summary TEXT NOT NULL DEFAULT '',
 semantic_version VARCHAR(64) NOT NULL,
 bundle_digest VARCHAR(64) NOT NULL,
 manifest_json TEXT NOT NULL,
 dependency_lock_json TEXT NOT NULL,
 bundle BLOB NOT NULL,
 introduced_by VARCHAR(255) NOT NULL DEFAULT '',
 introduced_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (public_release_id) REFERENCES public_agent_releases(id),
 FOREIGN KEY (public_listing_id) REFERENCES public_marketplace_listings(id)
);
CREATE UNIQUE INDEX uq_tenant_introduced_release_scope ON tenant_introduced_releases(tenant_id, public_release_id);
-- FK relaxation: sqlite cannot DROP CONSTRAINT, so rebuild both #59 tables
-- from their 000113 DDL minus the dropped FKs. The PRAGMAs at the file top
-- are only effective because internal/database/migration.go executes THIS
-- file through the dedicated NoTxWrap three-phase block (Task 2 Step 4);
-- under a per-file transaction wrapper they would be silent no-ops and the
-- DROP TABLE below would break FK-on deployments carrying adoption rows.
CREATE TABLE agent_adoptions_rebuild (
 id VARCHAR(36) NOT NULL, tenant_id INTEGER NOT NULL, listing_id VARCHAR(36) NOT NULL,
 accepted_release_id VARCHAR(36) NOT NULL, state VARCHAR(32) NOT NULL DEFAULT 'active',
 created_by VARCHAR(255) NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (id, tenant_id)
);
INSERT INTO agent_adoptions_rebuild (id, tenant_id, listing_id, accepted_release_id, state, created_by, created_at, updated_at)
 SELECT id, tenant_id, listing_id, accepted_release_id, state, created_by, created_at, updated_at FROM agent_adoptions;
DROP TABLE agent_adoptions;
ALTER TABLE agent_adoptions_rebuild RENAME TO agent_adoptions;
CREATE UNIQUE INDEX uq_agent_adoptions_scope ON agent_adoptions(tenant_id, listing_id);
CREATE TABLE agent_adoption_variants_rebuild (
 id VARCHAR(36) NOT NULL, tenant_id INTEGER NOT NULL, adoption_id VARCHAR(36) NOT NULL, release_id VARCHAR(36) NOT NULL,
 name VARCHAR(255) NOT NULL, state VARCHAR(32) NOT NULL DEFAULT 'draft',
 local_agent_id VARCHAR(36), local_agent_version_id VARCHAR(36),
 created_by VARCHAR(255) NOT NULL DEFAULT '', tested_by VARCHAR(255) NOT NULL DEFAULT '', tested_at DATETIME,
 published_by VARCHAR(255) NOT NULL DEFAULT '', published_at DATETIME,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (adoption_id, tenant_id) REFERENCES agent_adoptions(id, tenant_id)
);
INSERT INTO agent_adoption_variants_rebuild (id, tenant_id, adoption_id, release_id, name, state, local_agent_id, local_agent_version_id, created_by, tested_by, tested_at, published_by, published_at, created_at, updated_at)
 SELECT id, tenant_id, adoption_id, release_id, name, state, local_agent_id, local_agent_version_id, created_by, tested_by, tested_at, published_by, published_at, created_at, updated_at FROM agent_adoption_variants;
DROP TABLE agent_adoption_variants;
ALTER TABLE agent_adoption_variants_rebuild RENAME TO agent_adoption_variants;
CREATE INDEX idx_agent_adoption_variants_adoption ON agent_adoption_variants(tenant_id, adoption_id, created_at);
COMMIT;
PRAGMA foreign_keys = ON;
