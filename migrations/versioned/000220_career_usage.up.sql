CREATE TABLE career_usage_reservations (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  operation VARCHAR(32) NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  cost_units BIGINT NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'reserved',
  period_start TIMESTAMPTZ NOT NULL,
  period_end TIMESTAMPTZ NOT NULL,
  lease_until TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  settled_at TIMESTAMPTZ,
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_usage_scope ON career_usage_reservations (tenant_id, user_id, status, period_start);
