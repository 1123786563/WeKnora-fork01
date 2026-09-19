-- Durable retry state belongs to the memory job rather than a worker process.
-- retry_attempt counts consumed attempts; max_attempts is persisted with the
-- job so a restart cannot silently make retry unbounded.
ALTER TABLE native_memory_jobs ADD COLUMN IF NOT EXISTS retry_attempt BIGINT NOT NULL DEFAULT 0;
ALTER TABLE native_memory_jobs ADD COLUMN IF NOT EXISTS max_attempts BIGINT NOT NULL DEFAULT 3;
ALTER TABLE native_memory_jobs ADD COLUMN IF NOT EXISTS retry_status VARCHAR(32) NOT NULL DEFAULT 'none';
ALTER TABLE native_memory_jobs ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ;
ALTER TABLE native_memory_jobs ADD COLUMN IF NOT EXISTS last_error TEXT;

ALTER TABLE native_memory_jobs
    ADD CONSTRAINT native_memory_jobs_retry_attempt_nonnegative CHECK (retry_attempt >= 0),
    ADD CONSTRAINT native_memory_jobs_retry_max_attempts_positive CHECK (max_attempts > 0),
    ADD CONSTRAINT native_memory_jobs_retry_attempt_bounded CHECK (retry_attempt <= max_attempts),
    ADD CONSTRAINT native_memory_jobs_retry_status_known CHECK (retry_status IN ('none', 'scheduled', 'exhausted'));

CREATE INDEX IF NOT EXISTS idx_native_memory_jobs_retry_due
    ON native_memory_jobs (status, retry_status, next_attempt_at, tenant_id, subject_id);
