-- Dropping SessionKey columns would make populated recovery jobs ambiguous.
CREATE TABLE native_memory_jobs_session_key_down_guard (
    must_be_empty INTEGER NOT NULL CHECK (must_be_empty = 1)
);
INSERT INTO native_memory_jobs_session_key_down_guard (must_be_empty)
SELECT CASE WHEN EXISTS (SELECT 1 FROM native_memory_jobs) THEN 0 ELSE 1 END;
DROP TABLE native_memory_jobs_session_key_down_guard;

DROP INDEX IF EXISTS idx_native_memory_jobs_session_scope;
ALTER TABLE native_memory_jobs DROP COLUMN session_id;
ALTER TABLE native_memory_jobs DROP COLUMN session_user_id;
ALTER TABLE native_memory_jobs DROP COLUMN session_app_name;
