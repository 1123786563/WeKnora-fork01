-- P1.4 binds newly enqueued memory jobs to the exact native SessionKey used
-- for recovery. Existing jobs predate that boundary, so these columns remain
-- nullable until their lifecycle is explicitly reconciled.
ALTER TABLE native_memory_jobs ADD COLUMN IF NOT EXISTS session_app_name VARCHAR(255);
ALTER TABLE native_memory_jobs ADD COLUMN IF NOT EXISTS session_user_id VARCHAR(255);
ALTER TABLE native_memory_jobs ADD COLUMN IF NOT EXISTS session_id VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_native_memory_jobs_session_scope
    ON native_memory_jobs (tenant_id, session_app_name, session_user_id, session_id, status, created_at);
