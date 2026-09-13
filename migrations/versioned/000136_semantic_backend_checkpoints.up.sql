-- W03: backend checkpoints for shadow-build sync and rollback catch-up.
ALTER TABLE semantic_backend_states ADD COLUMN IF NOT EXISTS native_checkpoint BIGINT NOT NULL DEFAULT 0;
ALTER TABLE semantic_backend_states ADD COLUMN IF NOT EXISTS semantic_checkpoint BIGINT NOT NULL DEFAULT 0;
ALTER TABLE semantic_backend_states ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '';
