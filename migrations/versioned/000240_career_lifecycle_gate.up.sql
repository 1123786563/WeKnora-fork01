CREATE TABLE career_lifecycle_gates (
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  phase VARCHAR(16) NOT NULL,
  deletion_request_id VARCHAR(128) NOT NULL DEFAULT '',
  deletion_fingerprint VARCHAR(64) NOT NULL DEFAULT '',
  PRIMARY KEY (tenant_id, user_id)
);
CREATE TABLE career_lifecycle_claims (
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  operation VARCHAR(32) NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  fingerprint VARCHAR(64) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (tenant_id, user_id, operation, request_id)
);
CREATE INDEX idx_career_lifecycle_claim_scope ON career_lifecycle_claims (tenant_id, user_id);
