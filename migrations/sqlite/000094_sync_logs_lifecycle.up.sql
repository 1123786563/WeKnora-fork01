-- SP2-a connectors governance: attempt-level liveness and cooperative cancel
-- state for sync runs (SQLite track mirrors versioned 000173 on PostgreSQL).
ALTER TABLE sync_logs ADD COLUMN heartbeat_at DATETIME NULL;
ALTER TABLE sync_logs ADD COLUMN asynq_task_id VARCHAR(64) NULL;
ALTER TABLE sync_logs ADD COLUMN cancel_requested BOOLEAN NOT NULL DEFAULT FALSE;
