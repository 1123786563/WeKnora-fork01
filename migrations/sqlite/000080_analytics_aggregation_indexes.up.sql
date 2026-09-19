-- SQLite dialect of PG 000159
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages (created_at);
CREATE INDEX IF NOT EXISTS idx_sessions_tenant_created_at ON sessions (tenant_id, created_at);
