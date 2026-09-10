-- Durable tRPC agent runs, checkpoints, and recovery journals.

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS engine_type VARCHAR(16) NOT NULL DEFAULT 'builtin',
    ADD COLUMN IF NOT EXISTS active_agent_run_id VARCHAR(64);

CREATE UNIQUE INDEX IF NOT EXISTS uq_sessions_tenant_id_id
    ON sessions (tenant_id, id);
CREATE INDEX IF NOT EXISTS idx_sessions_active_agent_run
    ON sessions (tenant_id, active_agent_run_id)
    WHERE active_agent_run_id IS NOT NULL;

CREATE TABLE agent_runs (
    tenant_id INTEGER NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    request_id VARCHAR(64) NOT NULL,
    assistant_message_id VARCHAR(36) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    engine_type VARCHAR(16) NOT NULL DEFAULT 'trpc',
    status VARCHAR(32) NOT NULL DEFAULT 'queued',
    wait_reason VARCHAR(64) NOT NULL DEFAULT '',
    snapshot JSONB NOT NULL,
    graph_version VARCHAR(64) NOT NULL DEFAULT '1',
    sdk_version VARCHAR(64) NOT NULL DEFAULT '',
    schema_version INTEGER NOT NULL DEFAULT 1,
    lease_owner VARCHAR(128) NOT NULL DEFAULT '',
    lease_until TIMESTAMPTZ,
    epoch BIGINT NOT NULL DEFAULT 0,
    revision BIGINT NOT NULL DEFAULT 0,
    max_rounds INTEGER NOT NULL DEFAULT 0,
    max_tool_calls INTEGER NOT NULL DEFAULT 0,
    token_budget BIGINT NOT NULL DEFAULT 0,
    deadline TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id),
    CONSTRAINT fk_agent_runs_session
        FOREIGN KEY (tenant_id, session_id)
        REFERENCES sessions (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT ck_agent_runs_engine CHECK (engine_type = 'trpc')
);

CREATE UNIQUE INDEX uq_agent_runs_request
    ON agent_runs (tenant_id, owner_id, request_id);
CREATE INDEX idx_agent_runs_recovery_scan
    ON agent_runs (status, lease_until, created_at);
CREATE INDEX idx_agent_runs_session
    ON agent_runs (tenant_id, session_id);

CREATE TABLE agent_run_checkpoints (
    tenant_id INTEGER NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    checkpoint_id VARCHAR(255) NOT NULL,
    parent_id VARCHAR(255) NOT NULL DEFAULT '',
    seq BIGINT NOT NULL,
    state JSONB NOT NULL,
    pending_writes JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, namespace, checkpoint_id),
    CONSTRAINT fk_agent_run_checkpoints_run
        FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE,
    CONSTRAINT uq_agent_run_checkpoints_seq
        UNIQUE (tenant_id, run_id, namespace, seq)
);

CREATE TABLE agent_tool_calls (
    tenant_id INTEGER NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    call_id VARCHAR(255) NOT NULL,
    call_seq BIGINT NOT NULL,
    tool_name VARCHAR(255) NOT NULL,
    tool_identity VARCHAR(512) NOT NULL,
    tool_version VARCHAR(128) NOT NULL DEFAULT '',
    args_hash VARCHAR(64) NOT NULL,
    args JSONB NOT NULL,
    recovery_policy VARCHAR(32) NOT NULL DEFAULT 'wait_user',
    idempotency_key VARCHAR(255) NOT NULL DEFAULT '',
    idempotency_expires_at TIMESTAMPTZ,
    status VARCHAR(32) NOT NULL DEFAULT 'planned',
    external_task_ref VARCHAR(512) NOT NULL DEFAULT '',
    result JSONB,
    result_ref VARCHAR(1024) NOT NULL DEFAULT '',
    output_files JSONB NOT NULL DEFAULT '[]'::jsonb,
    source VARCHAR(32) NOT NULL DEFAULT '',
    unknown_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, call_id),
    CONSTRAINT fk_agent_tool_calls_run
        FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE,
    CONSTRAINT uq_agent_tool_calls_seq UNIQUE (tenant_id, run_id, call_seq)
);

CREATE TABLE agent_tool_attempts (
    tenant_id INTEGER NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    call_id VARCHAR(255) NOT NULL,
    attempt INTEGER NOT NULL,
    epoch BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL,
    dispatched_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    error_message TEXT NOT NULL DEFAULT '',
    external_credential TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, call_id, attempt),
    CONSTRAINT fk_agent_tool_attempts_call
        FOREIGN KEY (tenant_id, run_id, call_id)
        REFERENCES agent_tool_calls (tenant_id, run_id, call_id) ON DELETE CASCADE
);

CREATE TABLE agent_run_events (
    tenant_id INTEGER NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    seq BIGINT NOT NULL,
    attempt_id VARCHAR(255) NOT NULL DEFAULT '',
    event_type VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, seq),
    CONSTRAINT fk_agent_run_events_run
        FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE
);

CREATE TABLE agent_run_decisions (
    tenant_id INTEGER NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    decision_id VARCHAR(255) NOT NULL,
    pending_id VARCHAR(255) NOT NULL,
    tool_call_id VARCHAR(255),
    expected_revision BIGINT NOT NULL,
    actor_id VARCHAR(512) NOT NULL,
    action VARCHAR(32) NOT NULL,
    result JSONB,
    reason TEXT NOT NULL DEFAULT '',
    applied BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    applied_at TIMESTAMPTZ,
    PRIMARY KEY (tenant_id, run_id, decision_id),
    CONSTRAINT fk_agent_run_decisions_run
        FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX uq_agent_run_decisions_applied_pending
    ON agent_run_decisions (tenant_id, run_id, pending_id)
    WHERE applied;

CREATE TABLE agent_run_inputs (
    tenant_id INTEGER NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    steer_id VARCHAR(255) NOT NULL,
    mode VARCHAR(32) NOT NULL,
    message JSONB NOT NULL,
    cursor BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMPTZ,
    PRIMARY KEY (tenant_id, run_id, steer_id),
    CONSTRAINT fk_agent_run_inputs_run
        FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE
);
