-- Per-run capture recovery lookup: the primary key starts with workspace_id,
-- so the tenant+run drain query otherwise scans the whole tenant prefix.
CREATE INDEX IF NOT EXISTS idx_craft_run_captures_tenant_run ON craft_run_captures (tenant_id, run_id);