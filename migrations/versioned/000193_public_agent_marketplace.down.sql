-- Down: rebuild the relaxed FKs (fails if platform-lineage Adoption rows
-- exist, which is expected for a down migration over live data) and drop
-- the public marketplace tables in reverse dependency order. The circular
-- FK the up twin added (listings.current_release_id → public_agent_releases)
-- must be dropped first, or the DROP TABLE below is rejected.
ALTER TABLE public_marketplace_listings DROP CONSTRAINT IF EXISTS fk_public_marketplace_current_release;
ALTER TABLE agent_adoption_variants ADD CONSTRAINT fk_agent_adoption_variants_release
 FOREIGN KEY (release_id, tenant_id) REFERENCES agent_releases(id, tenant_id);
ALTER TABLE agent_adoptions ADD CONSTRAINT fk_agent_adoptions_release
 FOREIGN KEY (accepted_release_id, tenant_id) REFERENCES agent_releases(id, tenant_id);
ALTER TABLE agent_adoptions ADD CONSTRAINT fk_agent_adoptions_listing
 FOREIGN KEY (listing_id, tenant_id) REFERENCES agent_marketplace_listings(id, tenant_id);
DROP TABLE IF EXISTS tenant_introduced_releases;
DROP TABLE IF EXISTS public_agent_releases;
DROP TABLE IF EXISTS public_release_reviews;
DROP TABLE IF EXISTS public_release_submissions;
DROP TABLE IF EXISTS public_marketplace_listings;
DROP TABLE IF EXISTS public_marketplace_verified_publishers;
