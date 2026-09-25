CREATE TABLE career_progress_events (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  application_id VARCHAR(36) NOT NULL,
  seq BIGINT NOT NULL,
  kind VARCHAR(32) NOT NULL,
  event_type VARCHAR(32) NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  occurred_at TIMESTAMPTZ NOT NULL,
  corrects_event_id VARCHAR(36) NOT NULL DEFAULT '',
  source TEXT NOT NULL,
  confirmer VARCHAR(512) NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  fingerprint VARCHAR(64) NOT NULL,
  receipt_body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, user_id, application_id, seq),
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_progress_scope ON career_progress_events (tenant_id, user_id, application_id);
