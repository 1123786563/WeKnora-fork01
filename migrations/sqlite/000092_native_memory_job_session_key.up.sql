-- P1.4 binds newly enqueued memory jobs to the exact native SessionKey used
-- for recovery. Existing jobs predate that boundary, so these columns remain
-- nullable until their lifecycle is explicitly reconciled.
ALTER TABLE native_memory_jobs ADD COLUMN session_app_name TEXT;
ALTER TABLE native_memory_jobs ADD COLUMN session_user_id TEXT;
ALTER TABLE native_memory_jobs ADD COLUMN session_id TEXT;
CREATE INDEX IF NOT EXISTS idx_native_memory_jobs_session_scope
    ON native_memory_jobs (tenant_id, session_app_name, session_user_id, session_id, status, created_at);
