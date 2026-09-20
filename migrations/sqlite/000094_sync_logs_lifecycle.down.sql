-- Rollback: 000094_sync_logs_lifecycle
ALTER TABLE sync_logs DROP COLUMN cancel_requested;
ALTER TABLE sync_logs DROP COLUMN asynq_task_id;
ALTER TABLE sync_logs DROP COLUMN heartbeat_at;
