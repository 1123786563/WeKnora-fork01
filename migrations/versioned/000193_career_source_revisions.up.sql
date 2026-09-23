ALTER TABLE career_proposals ADD COLUMN evidence TEXT NOT NULL DEFAULT '';
CREATE TABLE career_source_revisions (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  revision BIGINT NOT NULL,
  file_name VARCHAR(255) NOT NULL,
  mime_type VARCHAR(128) NOT NULL,
  size BIGINT NOT NULL,
  digest VARCHAR(64) NOT NULL,
  request_id VARCHAR(128) NOT NULL DEFAULT '',
  intent_hash VARCHAR(64) NOT NULL DEFAULT '',
  expected_revision BIGINT NOT NULL DEFAULT 0,
  lease_until TIMESTAMPTZ,
  resource_ref TEXT NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL,
  error_category VARCHAR(64) NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  extracted_text TEXT NOT NULL DEFAULT '',
  missing_categories TEXT NOT NULL DEFAULT '[]',
  review_flags TEXT NOT NULL DEFAULT '[]',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  completed_at TIMESTAMPTZ,
  UNIQUE (tenant_id, user_id, revision)
);
CREATE INDEX idx_career_source_revisions_status ON career_source_revisions (status);
CREATE UNIQUE INDEX career_source_request_scope ON career_source_revisions (tenant_id, user_id, request_id) WHERE request_id <> '';
