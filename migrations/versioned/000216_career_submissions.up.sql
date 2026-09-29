CREATE TABLE career_submissions (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  application_id VARCHAR(36) NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  fingerprint VARCHAR(64) NOT NULL,
  channel VARCHAR(16) NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL,
  version_confirmed BOOLEAN NOT NULL DEFAULT FALSE,
  material_id VARCHAR(36) NOT NULL DEFAULT '',
  export_id VARCHAR(36) NOT NULL DEFAULT '',
  version BIGINT NOT NULL DEFAULT 0,
  content_digest VARCHAR(64) NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '',
  confirmer VARCHAR(512) NOT NULL,
  receipt_body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, user_id, application_id),
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_submission_scope ON career_submissions (tenant_id, user_id, application_id);
