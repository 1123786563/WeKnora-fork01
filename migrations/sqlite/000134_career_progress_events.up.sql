CREATE TABLE career_progress_events (
  id TEXT PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  application_id TEXT NOT NULL,
  seq INTEGER NOT NULL,
  kind TEXT NOT NULL,
  event_type TEXT NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  occurred_at DATETIME NOT NULL,
  corrects_event_id TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL,
  confirmer TEXT NOT NULL,
  request_id TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  receipt_body TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  UNIQUE (tenant_id, user_id, application_id, seq),
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_progress_scope ON career_progress_events (tenant_id, user_id, application_id);
