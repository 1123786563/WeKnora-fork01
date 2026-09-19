-- Retry state is recovery data. Do not discard it while any memory job exists.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM native_memory_jobs) THEN
        RAISE EXCEPTION 'native memory job retry rollback refused: table contains data';
    END IF;
END $$;

DROP INDEX IF EXISTS idx_native_memory_jobs_retry_due;
ALTER TABLE native_memory_jobs
    DROP CONSTRAINT IF EXISTS native_memory_jobs_retry_status_known,
    DROP CONSTRAINT IF EXISTS native_memory_jobs_retry_attempt_bounded,
    DROP CONSTRAINT IF EXISTS native_memory_jobs_retry_max_attempts_positive,
    DROP CONSTRAINT IF EXISTS native_memory_jobs_retry_attempt_nonnegative;
ALTER TABLE native_memory_jobs DROP COLUMN IF EXISTS last_error;
ALTER TABLE native_memory_jobs DROP COLUMN IF EXISTS next_attempt_at;
ALTER TABLE native_memory_jobs DROP COLUMN IF EXISTS retry_status;
ALTER TABLE native_memory_jobs DROP COLUMN IF EXISTS max_attempts;
ALTER TABLE native_memory_jobs DROP COLUMN IF EXISTS retry_attempt;
