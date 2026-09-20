-- Rollback: 000173_sync_logs_lifecycle
ALTER TABLE sync_logs DROP COLUMN IF EXISTS cancel_requested;
ALTER TABLE sync_logs DROP COLUMN IF EXISTS asynq_task_id;
ALTER TABLE sync_logs DROP COLUMN IF EXISTS heartbeat_at;
