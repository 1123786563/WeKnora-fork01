CREATE TABLE career_lifecycle_gates (
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  phase TEXT NOT NULL,
  deletion_request_id TEXT NOT NULL DEFAULT '',
  deletion_fingerprint TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (tenant_id, user_id)
);
CREATE TABLE career_lifecycle_claims (
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  operation TEXT NOT NULL,
  request_id TEXT NOT NULL,
  fingerprint TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  PRIMARY KEY (tenant_id, user_id, operation, request_id)
);
CREATE INDEX idx_career_lifecycle_claim_scope ON career_lifecycle_claims (tenant_id, user_id);
