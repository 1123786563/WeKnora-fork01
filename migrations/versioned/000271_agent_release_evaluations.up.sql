CREATE TABLE agent_release_evaluations (
 id VARCHAR(36) PRIMARY KEY,
 release_id VARCHAR(36) NOT NULL REFERENCES public_agent_releases(id) ON DELETE RESTRICT,
 test_set_id VARCHAR(255) NOT NULL,
 test_set_version VARCHAR(128) NOT NULL,
 environment_class VARCHAR(64) NOT NULL,
 evaluator_id VARCHAR(255) NOT NULL,
 evaluated_at TIMESTAMPTZ NOT NULL,
 results_json TEXT NOT NULL
);
CREATE UNIQUE INDEX uq_agent_release_evaluation_identity ON agent_release_evaluations(release_id, test_set_id, test_set_version, environment_class, evaluator_id);
CREATE INDEX idx_agent_release_evaluations_release ON agent_release_evaluations(release_id, evaluated_at DESC);
