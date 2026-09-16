CREATE TABLE execution_dispatches (
    tenant_id INTEGER NOT NULL,
    command_id VARCHAR(255) NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    attempt_id VARCHAR(255) NOT NULL,
    payload_hash VARCHAR(128) NOT NULL DEFAULT '',
    state VARCHAR(32) NOT NULL CHECK (state IN ('claimed', 'unknown', 'reconciled', 'completed')),
    external_id VARCHAR(512) NOT NULL DEFAULT '',
    worker VARCHAR(128) NOT NULL,
    epoch BIGINT NOT NULL,
    lease_until DATETIME,
    observed_state VARCHAR(64) NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, command_id),
    FOREIGN KEY (tenant_id, run_id) REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE
);
CREATE INDEX idx_execution_dispatches_recovery ON execution_dispatches (state, lease_until, updated_at);
CREATE INDEX idx_execution_dispatches_run ON execution_dispatches (tenant_id, run_id, updated_at);
