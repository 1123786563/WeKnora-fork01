-- T30 Public Marketplace: platform-scope listings/submissions/reviews/releases,
-- the Verified Publisher registry and the adopter-side introduction ledger.
-- Cross-tenant Adoptions reference platform-lineage ids (the public listing
-- and the introduced release), so the three FKs from the #59 governance
-- tables to agent_marketplace_listings/agent_releases are relaxed here;
-- tenant isolation stays enforced by every query's tenant_id binding.
CREATE TABLE public_marketplace_verified_publishers (
 tenant_id BIGINT PRIMARY KEY,
 state VARCHAR(32) NOT NULL DEFAULT 'verified',
 verified_by VARCHAR(255) NOT NULL DEFAULT '',
 note TEXT NOT NULL DEFAULT '',
 verified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE public_marketplace_listings (
 id VARCHAR(36) PRIMARY KEY,
 publisher_tenant_id BIGINT NOT NULL,
 source_listing_id VARCHAR(36) NOT NULL,
 display_name VARCHAR(255) NOT NULL,
 summary TEXT NOT NULL DEFAULT '',
 state VARCHAR(32) NOT NULL DEFAULT 'listed',
 current_release_id VARCHAR(36),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE (publisher_tenant_id, source_listing_id)
);
CREATE TABLE public_release_submissions (
 id VARCHAR(36) PRIMARY KEY,
 publisher_tenant_id BIGINT NOT NULL,
 public_listing_id VARCHAR(36) NOT NULL,
 source_listing_id VARCHAR(36) NOT NULL,
 source_release_id VARCHAR(36) NOT NULL,
 publisher_actor_id VARCHAR(255) NOT NULL DEFAULT '',
 semantic_version VARCHAR(64) NOT NULL,
 bundle_digest VARCHAR(64) NOT NULL,
 manifest_json TEXT NOT NULL,
 dependency_lock_json TEXT NOT NULL,
 bundle BYTEA NOT NULL,
 status VARCHAR(32) NOT NULL DEFAULT 'submitted',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
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
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 FOREIGN KEY (submission_id) REFERENCES public_release_submissions(id)
);
CREATE TABLE public_agent_releases (
 id VARCHAR(36) PRIMARY KEY,
 listing_id VARCHAR(36) NOT NULL,
 submission_id VARCHAR(36) NOT NULL,
 publisher_tenant_id BIGINT NOT NULL,
 release_number INTEGER NOT NULL CHECK (release_number >= 1),
 semantic_version VARCHAR(64) NOT NULL,
 bundle_digest VARCHAR(64) NOT NULL,
 manifest_json TEXT NOT NULL,
 dependency_lock_json TEXT NOT NULL,
 bundle BYTEA NOT NULL,
 published_by VARCHAR(255) NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 FOREIGN KEY (listing_id) REFERENCES public_marketplace_listings(id),
 FOREIGN KEY (submission_id) REFERENCES public_release_submissions(id)
);
ALTER TABLE public_marketplace_listings ADD CONSTRAINT fk_public_marketplace_current_release
 FOREIGN KEY (current_release_id) REFERENCES public_agent_releases(id);
CREATE INDEX idx_public_release_submissions_queue ON public_release_submissions(status, created_at);
CREATE INDEX idx_public_release_reviews_submission ON public_release_reviews(submission_id, created_at);
CREATE INDEX idx_public_agent_releases_listing ON public_agent_releases(listing_id, release_number);
CREATE UNIQUE INDEX uq_public_release_review_submission ON public_release_reviews(submission_id);
CREATE UNIQUE INDEX uq_public_agent_releases_number ON public_agent_releases(listing_id, release_number);
CREATE UNIQUE INDEX uq_public_agent_releases_semantic ON public_agent_releases(listing_id, semantic_version);
CREATE UNIQUE INDEX uq_public_agent_releases_digest ON public_agent_releases(listing_id, bundle_digest);
CREATE TABLE tenant_introduced_releases (
 id VARCHAR(36) NOT NULL,
 tenant_id BIGINT NOT NULL,
 public_listing_id VARCHAR(36) NOT NULL,
 public_release_id VARCHAR(36) NOT NULL,
 display_name VARCHAR(255) NOT NULL,
 summary TEXT NOT NULL DEFAULT '',
 semantic_version VARCHAR(64) NOT NULL,
 bundle_digest VARCHAR(64) NOT NULL,
 manifest_json TEXT NOT NULL,
 dependency_lock_json TEXT NOT NULL,
 bundle BYTEA NOT NULL,
 introduced_by VARCHAR(255) NOT NULL DEFAULT '',
 introduced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (public_release_id) REFERENCES public_agent_releases(id),
 FOREIGN KEY (public_listing_id) REFERENCES public_marketplace_listings(id)
);
CREATE UNIQUE INDEX uq_tenant_introduced_release_scope ON tenant_introduced_releases(tenant_id, public_release_id);
DO $$
DECLARE c text;
BEGIN
	SELECT conname INTO c FROM pg_constraint WHERE conrelid = 'agent_adoptions'::regclass AND pg_get_constraintdef(oid) ILIKE '%REFERENCES agent_marketplace_listings%';
	IF c IS NOT NULL THEN EXECUTE format('ALTER TABLE agent_adoptions DROP CONSTRAINT %I', c); END IF;
	SELECT conname INTO c FROM pg_constraint WHERE conrelid = 'agent_adoptions'::regclass AND pg_get_constraintdef(oid) ILIKE '%REFERENCES agent_releases%';
	IF c IS NOT NULL THEN EXECUTE format('ALTER TABLE agent_adoptions DROP CONSTRAINT %I', c); END IF;
	SELECT conname INTO c FROM pg_constraint WHERE conrelid = 'agent_adoption_variants'::regclass AND pg_get_constraintdef(oid) ILIKE '%REFERENCES agent_releases%';
	IF c IS NOT NULL THEN EXECUTE format('ALTER TABLE agent_adoption_variants DROP CONSTRAINT %I', c); END IF;
END $$;
