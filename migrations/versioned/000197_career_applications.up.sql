CREATE TABLE career_applications (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  fingerprint VARCHAR(64) NOT NULL,
  opportunity_id VARCHAR(36) NOT NULL,
  snapshot_id VARCHAR(36) NOT NULL,
  evaluation_id VARCHAR(36) NOT NULL,
  profile_revision BIGINT NOT NULL,
  evidence_body TEXT NOT NULL,
  batch_identity VARCHAR(255) NOT NULL,
  continue_despite_hard_failure BOOLEAN NOT NULL,
  evaluation_status VARCHAR(32) NOT NULL,
  qualified BOOLEAN NOT NULL,
  warning_body TEXT NOT NULL,
  link_state VARCHAR(32) NOT NULL,
  task_id VARCHAR(36) NOT NULL DEFAULT '',
  run_id VARCHAR(64) NOT NULL DEFAULT '',
  receipt_body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, user_id, request_id),
  UNIQUE (tenant_id, user_id, opportunity_id, batch_identity)
);
CREATE INDEX idx_career_applications_scope ON career_applications (tenant_id, user_id);
CREATE INDEX idx_career_applications_link_state ON career_applications (tenant_id, user_id, link_state);
