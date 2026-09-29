CREATE TABLE career_submissions (
  id TEXT PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  application_id TEXT NOT NULL,
  request_id TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  channel TEXT NOT NULL,
  occurred_at DATETIME NOT NULL,
  version_confirmed BOOLEAN NOT NULL DEFAULT 0,
  material_id TEXT NOT NULL DEFAULT '',
  export_id TEXT NOT NULL DEFAULT '',
  version INTEGER NOT NULL DEFAULT 0,
  content_digest TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '',
  confirmer TEXT NOT NULL,
  receipt_body TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE (tenant_id, user_id, application_id),
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_submission_scope ON career_submissions (tenant_id, user_id, application_id);
