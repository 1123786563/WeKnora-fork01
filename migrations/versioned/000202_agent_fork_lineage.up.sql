-- T32 Agent Fork lineage + license registry (#62): derived submissions and
-- releases record their source lineage and fork verdict; the source
-- license's redistribution flag gates re-submission (spec §5, §9).
-- Versioned twin of sqlite migration 000122.
ALTER TABLE agent_release_submissions ADD COLUMN is_fork BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE agent_release_submissions ADD COLUMN fork_source_listing_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_release_submissions ADD COLUMN fork_source_release_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_release_submissions ADD COLUMN fork_notes TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_release_submissions ADD COLUMN lineage_license_id VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN is_fork BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE agent_releases ADD COLUMN fork_source_listing_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN fork_source_release_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN fork_notes TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN lineage_license_id VARCHAR(64) NOT NULL DEFAULT '';
CREATE TABLE agent_licenses (
	id VARCHAR(64) NOT NULL,
	name VARCHAR(255) NOT NULL DEFAULT '',
	allows_redistribution BOOLEAN NOT NULL DEFAULT FALSE,
	created_by VARCHAR(255) NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (id)
);
