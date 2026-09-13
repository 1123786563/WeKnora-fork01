-- R02 down: remove only the Craft tables; sessions, runs and tool calls
-- belong to the recovery program and are left untouched.
DROP TABLE IF EXISTS craft_delegations;
DROP TABLE IF EXISTS craft_workspaces;
