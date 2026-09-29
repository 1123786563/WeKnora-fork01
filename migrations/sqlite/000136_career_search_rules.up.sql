CREATE TABLE career_search_rules (
  id TEXT PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  query TEXT NOT NULL,
  interval_minutes INTEGER NOT NULL,
  status TEXT NOT NULL,
  revision INTEGER NOT NULL DEFAULT 1,
  last_period INTEGER NOT NULL DEFAULT 0,
  next_due_at DATETIME NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE (tenant_id, user_id, id)
);
CREATE INDEX idx_career_search_rule_due ON career_search_rules (tenant_id, user_id, status, next_due_at);

CREATE TABLE career_search_rule_receipts (
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  request_id TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  body TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  UNIQUE (tenant_id, user_id, request_id)
);

CREATE TABLE career_search_rule_runs (
  id TEXT PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  rule_id TEXT NOT NULL,
  period INTEGER NOT NULL,
  request_id TEXT NOT NULL,
  status TEXT NOT NULL,
  body TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  UNIQUE (tenant_id, user_id, rule_id, period)
);
CREATE INDEX idx_career_search_rule_run_scope ON career_search_rule_runs (tenant_id, user_id, created_at);

CREATE TABLE career_search_discovery_todos (
  id TEXT PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  rule_id TEXT NOT NULL,
  run_id TEXT NOT NULL,
  search_id TEXT NOT NULL,
  source_id TEXT NOT NULL DEFAULT '',
  link TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'open',
  created_at DATETIME NOT NULL,
  UNIQUE (tenant_id, user_id, link)
);
CREATE INDEX idx_career_search_discovery_todo_rule ON career_search_discovery_todos (tenant_id, user_id, rule_id);
