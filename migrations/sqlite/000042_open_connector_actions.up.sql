-- T09: open-connector approval snapshot columns (sqlite twin of PG
-- 000122, offset -80). Same semantics: existing rows keep their legacy
-- digest and are marked generation 1 by the DEFAULT; new writes carry
-- generation 2 from the code path.
ALTER TABLE app_actions ADD COLUMN oc_binding_json TEXT NOT NULL DEFAULT '';
ALTER TABLE app_actions ADD COLUMN digest_version INTEGER NOT NULL DEFAULT 1;
