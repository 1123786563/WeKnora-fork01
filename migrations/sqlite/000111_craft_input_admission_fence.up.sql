-- T01 durable input-admission claim fields. Legacy rows remain fail-closed.
ALTER TABLE craft_session_requests ADD COLUMN admission_run_id VARCHAR(64);
ALTER TABLE craft_session_requests ADD COLUMN admission_token VARCHAR(64);
ALTER TABLE craft_session_requests ADD COLUMN admission_state VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE craft_session_requests ADD COLUMN lease_expires_at DATETIME;
