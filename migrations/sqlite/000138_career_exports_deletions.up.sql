-- T22: complete job-search data export and deletion ledger (SQLite twin).
CREATE TABLE career_data_exports (
  id TEXT PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  request_id TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  revision INTEGER NOT NULL,
  digest TEXT NOT NULL,
  archive_body TEXT NOT NULL,
  receipt_body TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_data_export_scope ON career_data_exports (tenant_id, user_id);

CREATE TABLE career_data_deletions (
  id TEXT PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  request_id TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  expected_revision INTEGER NOT NULL,
  status TEXT NOT NULL,
  state_body TEXT NOT NULL,
  receipt_body TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_data_deletion_scope ON career_data_deletions (tenant_id, user_id);
