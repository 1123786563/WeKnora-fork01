CREATE TABLE career_searches (
  id TEXT PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  request_id TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  status TEXT NOT NULL,
  claim_token TEXT NOT NULL DEFAULT '',
  lease_until DATETIME,
  query TEXT NOT NULL,
  receipt_body TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  completed_at DATETIME,
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_search_scope ON career_searches (tenant_id, user_id);
CREATE INDEX idx_career_search_status ON career_searches (tenant_id, user_id, status);
CREATE TABLE career_search_results (
  id TEXT PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  search_id TEXT NOT NULL,
  source_id TEXT NOT NULL DEFAULT '',
  link TEXT NOT NULL,
  checked_at DATETIME NOT NULL,
  qualification TEXT NOT NULL,
  uncertainty TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  UNIQUE (tenant_id, user_id, search_id, link)
);
CREATE INDEX idx_career_search_result_scope ON career_search_results (tenant_id, user_id, search_id);
