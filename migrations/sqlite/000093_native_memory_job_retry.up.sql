-- Durable retry state belongs to the memory job rather than a worker process.
-- retry_attempt counts consumed attempts; max_attempts is persisted with the
-- job so a restart cannot silently make retry unbounded.
ALTER TABLE native_memory_jobs ADD COLUMN retry_attempt INTEGER NOT NULL DEFAULT 0 CHECK (retry_attempt >= 0);
ALTER TABLE native_memory_jobs ADD COLUMN max_attempts INTEGER NOT NULL DEFAULT 3 CHECK (max_attempts > 0);
ALTER TABLE native_memory_jobs ADD COLUMN retry_status TEXT NOT NULL DEFAULT 'none'
    CHECK (retry_status IN ('none', 'scheduled', 'exhausted'));
ALTER TABLE native_memory_jobs ADD COLUMN next_attempt_at DATETIME;
ALTER TABLE native_memory_jobs ADD COLUMN last_error TEXT;

CREATE TRIGGER native_memory_jobs_retry_attempt_insert_guard
BEFORE INSERT ON native_memory_jobs
WHEN NEW.retry_attempt > NEW.max_attempts
BEGIN
    SELECT RAISE(ABORT, 'native memory job retry_attempt exceeds max_attempts');
END;

CREATE TRIGGER native_memory_jobs_retry_attempt_update_guard
BEFORE UPDATE OF retry_attempt, max_attempts ON native_memory_jobs
WHEN NEW.retry_attempt > NEW.max_attempts
BEGIN
    SELECT RAISE(ABORT, 'native memory job retry_attempt exceeds max_attempts');
END;

CREATE INDEX IF NOT EXISTS idx_native_memory_jobs_retry_due
    ON native_memory_jobs (status, retry_status, next_attempt_at, tenant_id, subject_id);
