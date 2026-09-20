-- T28 Tenant Catalog append-only submissions/reviews/releases and mutable listing pointer.
CREATE UNIQUE INDEX uq_agent_versions_source_binding ON agent_versions (tenant_id, id, agent_id);

CREATE TABLE agent_marketplace_listings (
 id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL, source_agent_id VARCHAR(36) NOT NULL,
 display_name VARCHAR(255) NOT NULL, summary TEXT NOT NULL DEFAULT '', state VARCHAR(32) NOT NULL DEFAULT 'listed',
 current_release_id VARCHAR(36), created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (id, tenant_id)
);
CREATE TABLE agent_release_submissions (
 id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL, listing_id VARCHAR(36) NOT NULL, agent_version_id VARCHAR(36) NOT NULL,
 source_agent_id VARCHAR(36) NOT NULL, author_id VARCHAR(255) NOT NULL DEFAULT '', semantic_version VARCHAR(64) NOT NULL,
 bundle_digest VARCHAR(64) NOT NULL, manifest_json TEXT NOT NULL, dependency_lock_json TEXT NOT NULL, bundle BYTEA NOT NULL,
 status VARCHAR(32) NOT NULL DEFAULT 'submitted', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (listing_id, tenant_id) REFERENCES agent_marketplace_listings(id, tenant_id),
 FOREIGN KEY (tenant_id, agent_version_id, source_agent_id) REFERENCES agent_versions(tenant_id, id, agent_id)
);
CREATE TABLE agent_release_reviews (
 id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL, submission_id VARCHAR(36) NOT NULL, reviewer_id VARCHAR(255) NOT NULL,
 reviewed_digest VARCHAR(64) NOT NULL, decision VARCHAR(32) NOT NULL, reason TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (id, tenant_id), FOREIGN KEY (submission_id, tenant_id) REFERENCES agent_release_submissions(id, tenant_id)
);
CREATE TABLE agent_releases (
 id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL, listing_id VARCHAR(36) NOT NULL, submission_id VARCHAR(36) NOT NULL,
 agent_version_id VARCHAR(36) NOT NULL, source_agent_id VARCHAR(36) NOT NULL, release_number INTEGER NOT NULL CHECK (release_number >= 1), semantic_version VARCHAR(64) NOT NULL,
 bundle_digest VARCHAR(64) NOT NULL, manifest_json TEXT NOT NULL, dependency_lock_json TEXT NOT NULL, bundle BYTEA NOT NULL,
 published_by VARCHAR(255) NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (listing_id, tenant_id) REFERENCES agent_marketplace_listings(id, tenant_id),
 FOREIGN KEY (submission_id, tenant_id) REFERENCES agent_release_submissions(id, tenant_id),
 FOREIGN KEY (tenant_id, agent_version_id, source_agent_id) REFERENCES agent_versions(tenant_id, id, agent_id)
);
ALTER TABLE agent_marketplace_listings ADD CONSTRAINT fk_agent_marketplace_current_release
 FOREIGN KEY (current_release_id, tenant_id) REFERENCES agent_releases(id, tenant_id);
CREATE INDEX idx_agent_release_submissions_review_queue ON agent_release_submissions(tenant_id, status, created_at);
CREATE INDEX idx_agent_release_reviews_submission ON agent_release_reviews(tenant_id, submission_id, created_at);
CREATE INDEX idx_agent_releases_listing ON agent_releases(tenant_id, listing_id, release_number);

CREATE UNIQUE INDEX uq_agent_marketplace_listing_scope ON agent_marketplace_listings(tenant_id, source_agent_id);
CREATE UNIQUE INDEX uq_agent_release_review_decision ON agent_release_reviews(submission_id, reviewer_id, reviewed_digest);
CREATE UNIQUE INDEX uq_agent_releases_number ON agent_releases(listing_id, release_number);
CREATE UNIQUE INDEX uq_agent_releases_semantic ON agent_releases(listing_id, semantic_version);
CREATE UNIQUE INDEX uq_agent_releases_digest ON agent_releases(listing_id, bundle_digest);
