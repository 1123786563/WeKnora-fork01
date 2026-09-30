ALTER TABLE career_opportunity_observations ADD COLUMN source_status TEXT NOT NULL DEFAULT '';
ALTER TABLE career_opportunity_observations ADD COLUMN completeness TEXT NOT NULL DEFAULT '';
ALTER TABLE career_opportunity_observations ADD COLUMN failure_code TEXT NOT NULL DEFAULT '';
ALTER TABLE career_opportunity_observations ADD COLUMN submitted_url TEXT NOT NULL DEFAULT '';
ALTER TABLE career_opportunity_observations ADD COLUMN final_url TEXT NOT NULL DEFAULT '';
ALTER TABLE career_opportunity_observations ADD COLUMN adapter_id TEXT NOT NULL DEFAULT '';
ALTER TABLE career_opportunity_observations ADD COLUMN adapter_version TEXT NOT NULL DEFAULT '';
ALTER TABLE career_opportunity_observations ADD COLUMN observed_http_status INTEGER NOT NULL DEFAULT 0;

CREATE INDEX idx_career_opportunity_observation_source ON career_opportunity_observations (tenant_id, user_id, source_kind, source_status);
