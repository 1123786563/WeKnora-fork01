-- W01 down: remove the Craft version tables in dependency order — the file
-- manifest first (it references versions), then the versions themselves.
-- Workspaces and delegations belong to R02 and stay untouched.
DROP TABLE IF EXISTS craft_version_files;
DROP TABLE IF EXISTS craft_versions;
