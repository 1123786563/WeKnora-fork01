-- Agent Fork lineage + license registry (T32, #62): derived submissions and
-- releases record their source lineage and fork verdict; the source
-- license's redistribution flag gates re-submission (spec §5, §9).
-- SQLite twin of versioned migration 000202.
ALTER TABLE agent_release_submissions ADD COLUMN is_fork BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE agent_release_submissions ADD COLUMN fork_source_listing_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_release_submissions ADD COLUMN fork_source_release_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_release_submissions ADD COLUMN fork_notes TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_release_submissions ADD COLUMN lineage_license_id VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN is_fork BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE agent_releases ADD COLUMN fork_source_listing_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN fork_source_release_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN fork_notes TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN lineage_license_id VARCHAR(64) NOT NULL DEFAULT '';
CREATE TABLE agent_licenses (
	id VARCHAR(64) NOT NULL,
	name VARCHAR(255) NOT NULL DEFAULT '',
	allows_redistribution BOOLEAN NOT NULL DEFAULT 0,
	created_by VARCHAR(255) NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id)
);
