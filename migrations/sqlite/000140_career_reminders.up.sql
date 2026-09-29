CREATE TABLE career_reminders (
  id TEXT PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  source_kind TEXT NOT NULL,
  source_id TEXT NOT NULL,
  application_id TEXT NOT NULL DEFAULT '',
  opportunity_id TEXT NOT NULL DEFAULT '',
  notice_key TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'open',
  request_id TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE (tenant_id, user_id, source_kind, source_id),
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_reminder_scope ON career_reminders (tenant_id, user_id, status, created_at);
CREATE TABLE career_reminder_receipts (
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  request_id TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  body TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  UNIQUE (tenant_id, user_id, request_id)
);
