-- T09: inverse drops of the approval-snapshot columns (sqlite twin of PG
-- 000122; the generation-2 data guard lives in the PG twin — sqlite has no
-- procedural blocks, matching the 000041 convention).
ALTER TABLE app_actions DROP COLUMN oc_binding_json;
ALTER TABLE app_actions DROP COLUMN digest_version;
