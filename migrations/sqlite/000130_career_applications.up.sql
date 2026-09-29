CREATE TABLE career_applications (
  id TEXT PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  request_id TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  opportunity_id TEXT NOT NULL,
  snapshot_id TEXT NOT NULL,
  evaluation_id TEXT NOT NULL,
  profile_revision INTEGER NOT NULL,
  evidence_body TEXT NOT NULL,
  batch_identity TEXT NOT NULL,
  continue_despite_hard_failure INTEGER NOT NULL,
  evaluation_status TEXT NOT NULL,
  qualified INTEGER NOT NULL,
  warning_body TEXT NOT NULL,
  link_state TEXT NOT NULL,
  task_id TEXT NOT NULL DEFAULT '',
  run_id TEXT NOT NULL DEFAULT '',
  receipt_body TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE (tenant_id, user_id, request_id),
  UNIQUE (tenant_id, user_id, opportunity_id, batch_identity)
);
CREATE INDEX idx_career_applications_scope ON career_applications (tenant_id, user_id);
CREATE INDEX idx_career_applications_link_state ON career_applications (tenant_id, user_id, link_state);
