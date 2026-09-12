-- T09: full approval snapshot for open-connector actions (PG; sqlite twin
-- is migrations/sqlite/000042). Adds the immutable OC execution binding
-- JSON and the digest generation column to app_actions. Pre-existing rows
-- keep their legacy digests and are marked generation 1 by the DEFAULT:
-- generation-1 rows in preparable states must be re-Prepared before an
-- approval can continue (the service refuses to approve or execute them);
-- terminal/unknown rows keep their old digest and recovery semantics
-- untouched. New writes carry generation 2 from the code path.
ALTER TABLE app_actions ADD COLUMN oc_binding_json TEXT NOT NULL DEFAULT '';
ALTER TABLE app_actions ADD COLUMN digest_version BIGINT NOT NULL DEFAULT 1;
