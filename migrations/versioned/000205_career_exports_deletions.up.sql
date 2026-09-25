-- T22: complete job-search data export and deletion ledger.
-- career_data_exports stores one full export package per request ID (the
-- frozen archive body plus its verifiable digest); career_data_deletions is
-- the recoverable deletion state machine and audit row.
CREATE TABLE career_data_exports (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  fingerprint VARCHAR(64) NOT NULL,
  revision BIGINT NOT NULL,
  digest VARCHAR(64) NOT NULL,
  archive_body TEXT NOT NULL,
  receipt_body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_data_export_scope ON career_data_exports (tenant_id, user_id);

CREATE TABLE career_data_deletions (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  fingerprint VARCHAR(64) NOT NULL,
  expected_revision BIGINT NOT NULL,
  status VARCHAR(16) NOT NULL,
  state_body TEXT NOT NULL,
  receipt_body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_data_deletion_scope ON career_data_deletions (tenant_id, user_id);
