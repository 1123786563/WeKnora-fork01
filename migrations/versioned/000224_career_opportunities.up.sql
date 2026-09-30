CREATE TABLE career_opportunities (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_career_opportunity_scope ON career_opportunities (tenant_id, user_id);

CREATE TABLE career_opportunity_observations (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  opportunity_id VARCHAR(36) NOT NULL,
  snapshot_id VARCHAR(36) NOT NULL,
  source_kind VARCHAR(32) NOT NULL,
  source_label TEXT NOT NULL DEFAULT '',
  source_ref TEXT NOT NULL DEFAULT '',
  acquired_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_career_opportunity_observation_scope ON career_opportunity_observations (tenant_id, user_id);
CREATE INDEX idx_career_opportunity_observations_opportunity_id ON career_opportunity_observations (opportunity_id);

CREATE TABLE career_opportunity_snapshots (
  id VARCHAR(36) PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  opportunity_id VARCHAR(36) NOT NULL,
  observation_id VARCHAR(36) NOT NULL,
  raw_text TEXT NOT NULL,
  raw_sha256 VARCHAR(64) NOT NULL,
  extracted TEXT NOT NULL,
  status VARCHAR(32) NOT NULL,
  acquired_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_career_opportunity_snapshot_scope ON career_opportunity_snapshots (tenant_id, user_id);
CREATE INDEX idx_career_opportunity_snapshots_opportunity_id ON career_opportunity_snapshots (opportunity_id);

CREATE TABLE career_opportunity_receipts (
  tenant_id BIGINT NOT NULL,
  user_id VARCHAR(512) NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  fingerprint VARCHAR(64) NOT NULL,
  body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, user_id, request_id)
);
