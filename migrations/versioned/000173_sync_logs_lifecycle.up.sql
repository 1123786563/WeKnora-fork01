-- SP2-a connectors governance: attempt-level liveness and cooperative cancel
-- state for sync runs. heartbeat_at NULL = no heartbeat received yet;
-- asynq_task_id links a run back to its queue task; cancel_requested is set
-- by the API and observed by the sync loop at checkpoint boundaries.
ALTER TABLE sync_logs ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMP NULL;
ALTER TABLE sync_logs ADD COLUMN IF NOT EXISTS asynq_task_id VARCHAR(64) NULL;
ALTER TABLE sync_logs ADD COLUMN IF NOT EXISTS cancel_requested BOOLEAN NOT NULL DEFAULT FALSE;

DO $$ BEGIN RAISE NOTICE '[Migration 000173] sync_logs lifecycle columns added (heartbeat_at, asynq_task_id, cancel_requested)'; END $$;
