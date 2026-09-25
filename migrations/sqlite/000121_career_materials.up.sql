CREATE TABLE career_materials (
  id TEXT PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  request_id TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  opportunity_id TEXT NOT NULL,
  snapshot_id TEXT NOT NULL,
  profile_revision INTEGER NOT NULL,
  evidence_body TEXT NOT NULL,
  status TEXT NOT NULL,
  draft_body TEXT NOT NULL,
  failure_code TEXT NOT NULL DEFAULT '',
  failure_message TEXT NOT NULL DEFAULT '',
  version_count INTEGER NOT NULL DEFAULT 0,
  receipt_body TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE (tenant_id, user_id, request_id)
);
CREATE INDEX idx_career_material_scope ON career_materials (tenant_id, user_id);
CREATE INDEX idx_career_material_status ON career_materials (tenant_id, user_id, status);
CREATE TABLE career_material_versions (
  id TEXT PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  material_id TEXT NOT NULL,
  version INTEGER NOT NULL,
  request_id TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  evidence_body TEXT NOT NULL,
  version_body TEXT NOT NULL,
  receipt_body TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  UNIQUE (tenant_id, user_id, material_id, version)
);
CREATE INDEX idx_career_material_version_scope ON career_material_versions (tenant_id, user_id);
CREATE TABLE career_material_receipts (
  tenant_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  request_id TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  body TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  UNIQUE (tenant_id, user_id, request_id)
);
