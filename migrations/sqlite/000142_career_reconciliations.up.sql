CREATE TABLE career_reconciliations (
  id TEXT PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  request_id TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  decision TEXT NOT NULL,
  target_id TEXT NOT NULL,
  candidate_id TEXT NOT NULL,
  evidence_body TEXT NOT NULL,
  receipt_body TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_reconciliation_scope ON career_reconciliations (tenant_id, user_id, created_at);
CREATE INDEX idx_career_reconciliation_target ON career_reconciliations (tenant_id, user_id, target_id);
CREATE INDEX idx_career_reconciliation_candidate ON career_reconciliations (tenant_id, user_id, candidate_id);
