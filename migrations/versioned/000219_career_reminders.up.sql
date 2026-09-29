CREATE TABLE career_reminders (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  source_kind VARCHAR(32) NOT NULL,
  source_id VARCHAR(36) NOT NULL,
  application_id VARCHAR(36) NOT NULL DEFAULT '',
  opportunity_id VARCHAR(36) NOT NULL DEFAULT '',
  notice_key VARCHAR(64) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'open',
  request_id VARCHAR(128) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, user_id, source_kind, source_id),
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_reminder_scope ON career_reminders (tenant_id, user_id, status, created_at);
CREATE TABLE career_reminder_receipts (
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  fingerprint VARCHAR(64) NOT NULL,
  body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, user_id, request_id)
);
