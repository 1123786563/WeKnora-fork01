CREATE TABLE career_materials (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  fingerprint VARCHAR(64) NOT NULL,
  opportunity_id VARCHAR(36) NOT NULL,
  snapshot_id VARCHAR(36) NOT NULL,
  profile_revision BIGINT NOT NULL,
  evidence_body TEXT NOT NULL,
  status VARCHAR(16) NOT NULL,
  draft_body TEXT NOT NULL,
  failure_code VARCHAR(64) NOT NULL DEFAULT '',
  failure_message TEXT NOT NULL DEFAULT '',
  version_count BIGINT NOT NULL DEFAULT 0,
  receipt_body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_material_scope ON career_materials (tenant_id, user_id);
CREATE INDEX idx_career_material_status ON career_materials (tenant_id, user_id, status);
CREATE TABLE career_material_versions (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  material_id VARCHAR(36) NOT NULL,
  version BIGINT NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  fingerprint VARCHAR(64) NOT NULL,
  evidence_body TEXT NOT NULL,
  version_body TEXT NOT NULL,
  receipt_body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, user_id, material_id, version)
);
CREATE INDEX idx_career_material_version_scope ON career_material_versions (tenant_id, user_id);
CREATE TABLE career_material_receipts (
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  fingerprint VARCHAR(64) NOT NULL,
  body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, user_id, request_id)
);
