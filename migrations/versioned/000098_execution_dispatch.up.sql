CREATE TABLE execution_dispatches (
    tenant_id BIGINT NOT NULL,
    command_id VARCHAR(255) NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    attempt_id VARCHAR(255) NOT NULL,
    payload_hash VARCHAR(128) NOT NULL DEFAULT '',
    state VARCHAR(32) NOT NULL,
    external_id VARCHAR(512) NOT NULL DEFAULT '',
    worker VARCHAR(128) NOT NULL,
    epoch BIGINT NOT NULL,
    lease_until TIMESTAMPTZ,
    observed_state VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, command_id),
    CONSTRAINT fk_execution_dispatches_run FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE,
    CONSTRAINT ck_execution_dispatches_state CHECK (state IN ('claimed', 'unknown', 'reconciled', 'completed'))
);
CREATE INDEX idx_execution_dispatches_recovery
    ON execution_dispatches (state, lease_until, updated_at);
CREATE INDEX idx_execution_dispatches_run
    ON execution_dispatches (tenant_id, run_id, updated_at);
