ALTER TABLE semantic_backend_states DROP COLUMN IF EXISTS last_error;
ALTER TABLE semantic_backend_states DROP COLUMN IF EXISTS semantic_checkpoint;
ALTER TABLE semantic_backend_states DROP COLUMN IF EXISTS native_checkpoint;
