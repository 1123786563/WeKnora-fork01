CREATE TABLE career_searches (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  fingerprint VARCHAR(64) NOT NULL,
  status VARCHAR(16) NOT NULL,
  claim_token VARCHAR(36) NOT NULL DEFAULT '',
  lease_until TIMESTAMPTZ,
  query VARCHAR(512) NOT NULL,
  receipt_body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  completed_at TIMESTAMPTZ,
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_search_scope ON career_searches (tenant_id, user_id);
CREATE INDEX idx_career_search_status ON career_searches (tenant_id, user_id, status);
CREATE TABLE career_search_results (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  search_id VARCHAR(36) NOT NULL,
  source_id VARCHAR(64) NOT NULL DEFAULT '',
  link VARCHAR(2048) NOT NULL,
  checked_at TIMESTAMPTZ NOT NULL,
  qualification VARCHAR(32) NOT NULL,
  uncertainty VARCHAR(32) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, user_id, search_id, link)
);
CREATE INDEX idx_career_search_result_scope ON career_search_results (tenant_id, user_id, search_id);
