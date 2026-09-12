-- T09: rollback of the approval-snapshot columns. Refuses to run once any
-- row was written by generation-2 code (a non-empty oc_binding_json or a
-- digest_version other than 1): dropping those columns would strip
-- approval-bound execution identity from live audit records. The
-- operational rollback path is disabling the feature, not deleting data.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM app_actions WHERE oc_binding_json <> '' OR digest_version <> 1) THEN
        RAISE EXCEPTION 'app_actions carries generation-2 approval snapshots: disable the feature instead of dropping approval-bound columns';
    END IF;
END
$$;

ALTER TABLE app_actions DROP COLUMN IF EXISTS oc_binding_json;
ALTER TABLE app_actions DROP COLUMN IF EXISTS digest_version;
