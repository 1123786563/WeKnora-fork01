CREATE TABLE execution_observations (
    tenant_id BIGINT NOT NULL, run_id VARCHAR(64) NOT NULL, binding_id VARCHAR(255) NOT NULL,
    generation VARCHAR(255) NOT NULL, event_id VARCHAR(255) NOT NULL, attempt_id VARCHAR(255) NOT NULL DEFAULT '',
    event_type VARCHAR(128) NOT NULL, payload_hash VARCHAR(128) NOT NULL, payload JSONB NOT NULL,
    product_seq BIGINT NOT NULL, source_seq BIGINT NOT NULL DEFAULT 0, history_incomplete BOOLEAN NOT NULL DEFAULT FALSE,
    confirmed_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, binding_id, generation, event_id),
    UNIQUE (tenant_id, run_id, product_seq),
    CONSTRAINT fk_execution_observations_run FOREIGN KEY (tenant_id, run_id) REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE
);
CREATE INDEX idx_execution_observations_run ON execution_observations (tenant_id, run_id, product_seq);
