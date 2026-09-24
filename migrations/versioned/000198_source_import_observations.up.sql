ALTER TABLE career_opportunity_observations
  ADD COLUMN source_status VARCHAR(32) NOT NULL DEFAULT '',
  ADD COLUMN completeness VARCHAR(16) NOT NULL DEFAULT '',
  ADD COLUMN failure_code VARCHAR(32) NOT NULL DEFAULT '',
  ADD COLUMN submitted_url TEXT NOT NULL DEFAULT '',
  ADD COLUMN final_url TEXT NOT NULL DEFAULT '',
  ADD COLUMN adapter_id VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN adapter_version VARCHAR(32) NOT NULL DEFAULT '',
  ADD COLUMN observed_http_status INTEGER NOT NULL DEFAULT 0;

CREATE INDEX idx_career_opportunity_observation_source ON career_opportunity_observations (tenant_id, user_id, source_kind, source_status);
