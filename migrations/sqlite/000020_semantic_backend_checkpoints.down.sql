-- SQLite has no DROP COLUMN IF EXISTS; the -4 step ordering guarantees
-- the columns exist when this down migration runs.
ALTER TABLE semantic_backend_states DROP COLUMN last_error;
ALTER TABLE semantic_backend_states DROP COLUMN semantic_checkpoint;
ALTER TABLE semantic_backend_states DROP COLUMN native_checkpoint;
