CREATE TABLE execution_observations (
    tenant_id INTEGER NOT NULL, run_id VARCHAR(64) NOT NULL, binding_id VARCHAR(255) NOT NULL,
    generation VARCHAR(255) NOT NULL, event_id VARCHAR(255) NOT NULL, attempt_id VARCHAR(255) NOT NULL DEFAULT '',
    event_type VARCHAR(128) NOT NULL, payload_hash VARCHAR(128) NOT NULL, payload JSON NOT NULL,
    product_seq INTEGER NOT NULL, source_seq INTEGER NOT NULL DEFAULT 0, history_incomplete INTEGER NOT NULL DEFAULT 0,
    confirmed_snapshot JSON NOT NULL DEFAULT '{}', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, binding_id, generation, event_id), UNIQUE (tenant_id, run_id, product_seq),
    FOREIGN KEY (tenant_id, run_id) REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE
);
CREATE INDEX idx_execution_observations_run ON execution_observations (tenant_id, run_id, product_seq);
CREATE TABLE execution_source_cursors (
    tenant_id INTEGER NOT NULL, binding_id VARCHAR(255) NOT NULL, generation VARCHAR(255) NOT NULL,
    last_confirmed_seq INTEGER NOT NULL DEFAULT 0, confirmed_snapshot JSON NOT NULL DEFAULT '[]',
    PRIMARY KEY (tenant_id, binding_id, generation)
);
