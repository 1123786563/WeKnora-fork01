-- Per-run capture recovery lookup (see the SQLite twin).
CREATE INDEX IF NOT EXISTS idx_craft_run_captures_tenant_run ON craft_run_captures (tenant_id, run_id);