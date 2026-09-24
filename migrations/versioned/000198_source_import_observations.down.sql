DROP INDEX IF EXISTS idx_career_opportunity_observation_source;
ALTER TABLE career_opportunity_observations
  DROP COLUMN IF EXISTS observed_http_status,
  DROP COLUMN IF EXISTS adapter_version,
  DROP COLUMN IF EXISTS adapter_id,
  DROP COLUMN IF EXISTS final_url,
  DROP COLUMN IF EXISTS submitted_url,
  DROP COLUMN IF EXISTS failure_code,
  DROP COLUMN IF EXISTS completeness,
  DROP COLUMN IF EXISTS source_status;
