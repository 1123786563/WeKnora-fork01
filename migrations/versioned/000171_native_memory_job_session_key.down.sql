-- Dropping SessionKey columns would make populated recovery jobs ambiguous.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM native_memory_jobs) THEN
        RAISE EXCEPTION 'native memory job SessionKey rollback refused: table contains data';
    END IF;
END $$;

DROP INDEX IF EXISTS idx_native_memory_jobs_session_scope;
ALTER TABLE native_memory_jobs DROP COLUMN IF EXISTS session_id;
ALTER TABLE native_memory_jobs DROP COLUMN IF EXISTS session_user_id;
ALTER TABLE native_memory_jobs DROP COLUMN IF EXISTS session_app_name;
