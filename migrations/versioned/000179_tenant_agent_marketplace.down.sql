ALTER TABLE agent_marketplace_listings DROP CONSTRAINT IF EXISTS fk_agent_marketplace_current_release;
DROP TABLE IF EXISTS agent_release_reviews;
DROP TABLE IF EXISTS agent_releases;
DROP TABLE IF EXISTS agent_release_submissions;
DROP TABLE IF EXISTS agent_marketplace_listings;
