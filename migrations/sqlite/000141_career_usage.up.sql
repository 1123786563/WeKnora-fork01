CREATE TABLE career_usage_reservations (
  id TEXT PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  operation TEXT NOT NULL,
  request_id TEXT NOT NULL,
  cost_units INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'reserved',
  period_start DATETIME NOT NULL,
  period_end DATETIME NOT NULL,
  lease_until DATETIME,
  created_at DATETIME NOT NULL,
  settled_at DATETIME,
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_usage_scope ON career_usage_reservations (tenant_id, user_id, status, period_start);
