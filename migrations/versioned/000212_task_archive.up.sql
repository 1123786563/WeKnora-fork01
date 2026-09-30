-- Task lifecycle archived state (T04). Task = Session is the single task
-- identity (ADR-0004), so the archive timestamp lives on the task itself.
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;
