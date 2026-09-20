UPDATE agent_marketplace_listings SET current_release_id = NULL;
DROP TABLE IF EXISTS agent_release_reviews;
DROP TABLE IF EXISTS agent_releases;
DROP TABLE IF EXISTS agent_release_submissions;
DROP TABLE IF EXISTS agent_marketplace_listings;
DROP INDEX IF EXISTS uq_agent_versions_source_binding;
