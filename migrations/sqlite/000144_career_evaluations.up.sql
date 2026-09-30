CREATE TABLE career_evaluations (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  fingerprint VARCHAR(64) NOT NULL,
  intent TEXT NOT NULL,
  opportunity_id VARCHAR(36) NOT NULL,
  snapshot_id VARCHAR(36) NOT NULL,
  profile_revision INTEGER NOT NULL,
  receipt_body TEXT NOT NULL,
  evaluation_body TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_evaluation_scope ON career_evaluations (tenant_id, user_id);
CREATE INDEX idx_career_evaluations_opportunity_id ON career_evaluations (opportunity_id);
CREATE INDEX idx_career_evaluations_snapshot_id ON career_evaluations (snapshot_id);
