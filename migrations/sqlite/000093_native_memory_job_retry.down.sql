-- Retry state is recovery data. Do not discard it while any memory job exists.
CREATE TABLE native_memory_jobs_retry_down_guard (
    must_be_empty INTEGER NOT NULL CHECK (must_be_empty = 1)
);
INSERT INTO native_memory_jobs_retry_down_guard (must_be_empty)
SELECT CASE WHEN EXISTS (SELECT 1 FROM native_memory_jobs) THEN 0 ELSE 1 END;
DROP TABLE native_memory_jobs_retry_down_guard;

DROP INDEX IF EXISTS idx_native_memory_jobs_retry_due;
DROP TRIGGER IF EXISTS native_memory_jobs_retry_attempt_update_guard;
DROP TRIGGER IF EXISTS native_memory_jobs_retry_attempt_insert_guard;
ALTER TABLE native_memory_jobs DROP COLUMN last_error;
ALTER TABLE native_memory_jobs DROP COLUMN next_attempt_at;
ALTER TABLE native_memory_jobs DROP COLUMN retry_status;
ALTER TABLE native_memory_jobs DROP COLUMN max_attempts;
ALTER TABLE native_memory_jobs DROP COLUMN retry_attempt;
