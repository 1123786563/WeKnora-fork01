ALTER TABLE semantic_backend_states ADD COLUMN native_checkpoint INTEGER NOT NULL DEFAULT 0;
ALTER TABLE semantic_backend_states ADD COLUMN semantic_checkpoint INTEGER NOT NULL DEFAULT 0;
ALTER TABLE semantic_backend_states ADD COLUMN last_error TEXT NOT NULL DEFAULT '';
