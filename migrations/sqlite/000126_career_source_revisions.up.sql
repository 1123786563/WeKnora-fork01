ALTER TABLE career_proposals ADD COLUMN evidence TEXT NOT NULL DEFAULT '';
CREATE TABLE career_source_revisions (
  id TEXT PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  revision INTEGER NOT NULL,
  file_name VARCHAR(255) NOT NULL,
  mime_type VARCHAR(128) NOT NULL,
  size INTEGER NOT NULL,
  digest VARCHAR(64) NOT NULL,
  request_id VARCHAR(128) NOT NULL DEFAULT '',
  intent_hash VARCHAR(64) NOT NULL DEFAULT '',
  expected_revision INTEGER NOT NULL DEFAULT 0,
  claim_token VARCHAR(36) NOT NULL DEFAULT '',
  lease_until DATETIME,
  resource_ref TEXT NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL,
  error_category VARCHAR(64) NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  extracted_text TEXT NOT NULL DEFAULT '',
  missing_categories TEXT NOT NULL DEFAULT '[]',
  review_flags TEXT NOT NULL DEFAULT '[]',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  completed_at DATETIME,
  UNIQUE (tenant_id, user_id, revision)
);
CREATE INDEX idx_career_source_revisions_status ON career_source_revisions (status);
CREATE UNIQUE INDEX career_source_request_scope ON career_source_revisions (tenant_id, user_id, request_id) WHERE request_id <> '';
