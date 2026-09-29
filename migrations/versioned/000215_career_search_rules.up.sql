CREATE TABLE career_search_rules (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  query VARCHAR(512) NOT NULL,
  interval_minutes BIGINT NOT NULL,
  status VARCHAR(16) NOT NULL,
  revision BIGINT NOT NULL DEFAULT 1,
  last_period BIGINT NOT NULL DEFAULT 0,
  next_due_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, user_id, id)
);
CREATE INDEX idx_career_search_rule_due ON career_search_rules (tenant_id, user_id, status, next_due_at);

CREATE TABLE career_search_rule_receipts (
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  fingerprint VARCHAR(64) NOT NULL,
  body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, user_id, request_id)
);

CREATE TABLE career_search_rule_runs (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  rule_id VARCHAR(36) NOT NULL,
  period BIGINT NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  status VARCHAR(32) NOT NULL,
  body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, user_id, rule_id, period)
);
CREATE INDEX idx_career_search_rule_run_scope ON career_search_rule_runs (tenant_id, user_id, created_at);

CREATE TABLE career_search_discovery_todos (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  rule_id VARCHAR(36) NOT NULL,
  run_id VARCHAR(36) NOT NULL,
  search_id VARCHAR(36) NOT NULL,
  source_id VARCHAR(64) NOT NULL DEFAULT '',
  link VARCHAR(2048) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'open',
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, user_id, link)
);
CREATE INDEX idx_career_search_discovery_todo_rule ON career_search_discovery_todos (tenant_id, user_id, rule_id);
