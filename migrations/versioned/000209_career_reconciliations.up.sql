CREATE TABLE career_reconciliations (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  fingerprint VARCHAR(64) NOT NULL,
  decision VARCHAR(16) NOT NULL,
  target_id VARCHAR(36) NOT NULL,
  candidate_id VARCHAR(36) NOT NULL,
  evidence_body TEXT NOT NULL,
  receipt_body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_reconciliation_scope ON career_reconciliations (tenant_id, user_id, created_at);
CREATE INDEX idx_career_reconciliation_target ON career_reconciliations (tenant_id, user_id, target_id);
CREATE INDEX idx_career_reconciliation_candidate ON career_reconciliations (tenant_id, user_id, candidate_id);
