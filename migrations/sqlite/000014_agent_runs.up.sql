-- Durable tRPC agent runs, checkpoints, and recovery journals.

ALTER TABLE sessions ADD COLUMN engine_type VARCHAR(16) NOT NULL DEFAULT 'builtin';
ALTER TABLE sessions ADD COLUMN active_agent_run_id VARCHAR(64);

CREATE UNIQUE INDEX uq_sessions_tenant_id_id
    ON sessions (tenant_id, id);
CREATE INDEX idx_sessions_active_agent_run
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
    engine_type VARCHAR(16) NOT NULL DEFAULT 'trpc' CHECK (engine_type = 'trpc'),
    status VARCHAR(32) NOT NULL DEFAULT 'queued',
    wait_reason VARCHAR(64) NOT NULL DEFAULT '',
    snapshot TEXT NOT NULL,
    graph_version VARCHAR(64) NOT NULL DEFAULT '1',
    sdk_version VARCHAR(64) NOT NULL DEFAULT '',
    schema_version INTEGER NOT NULL DEFAULT 1,
    lease_owner VARCHAR(128) NOT NULL DEFAULT '',
    lease_until DATETIME,
    epoch BIGINT NOT NULL DEFAULT 0,
    revision BIGINT NOT NULL DEFAULT 0,
    max_rounds INTEGER NOT NULL DEFAULT 0,
    max_tool_calls INTEGER NOT NULL DEFAULT 0,
    token_budget BIGINT NOT NULL DEFAULT 0,
    deadline DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id),
    FOREIGN KEY (tenant_id, session_id)
        REFERENCES sessions (tenant_id, id) ON DELETE CASCADE
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
    state TEXT NOT NULL,
    pending_writes TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, namespace, checkpoint_id),
    UNIQUE (tenant_id, run_id, namespace, seq),
    FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE
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
    args TEXT NOT NULL,
    recovery_policy VARCHAR(32) NOT NULL DEFAULT 'wait_user',
    idempotency_key VARCHAR(255) NOT NULL DEFAULT '',
    idempotency_expires_at DATETIME,
    status VARCHAR(32) NOT NULL DEFAULT 'planned',
    external_task_ref VARCHAR(512) NOT NULL DEFAULT '',
    result TEXT,
    result_ref VARCHAR(1024) NOT NULL DEFAULT '',
    output_files TEXT NOT NULL DEFAULT '[]',
    source VARCHAR(32) NOT NULL DEFAULT '',
    unknown_reason TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, call_id),
    UNIQUE (tenant_id, run_id, call_seq),
    FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE
);

CREATE TABLE agent_tool_attempts (
    tenant_id INTEGER NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    call_id VARCHAR(255) NOT NULL,
    attempt INTEGER NOT NULL,
    epoch BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL,
    dispatched_at DATETIME,
    finished_at DATETIME,
    error_message TEXT NOT NULL DEFAULT '',
    external_credential TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, call_id, attempt),
    FOREIGN KEY (tenant_id, run_id, call_id)
        REFERENCES agent_tool_calls (tenant_id, run_id, call_id) ON DELETE CASCADE
);

CREATE TABLE agent_run_events (
    tenant_id INTEGER NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    seq BIGINT NOT NULL,
    attempt_id VARCHAR(255) NOT NULL DEFAULT '',
    event_type VARCHAR(128) NOT NULL,
    payload TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, seq),
    FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE
);

CREATE TABLE agent_run_decisions (
    tenant_id INTEGER NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    decision_id VARCHAR(255) NOT NULL,
    pending_id VARCHAR(255) NOT NULL,
    tool_call_id VARCHAR(255),
    args_hash VARCHAR(64) NOT NULL DEFAULT '',
    resource_ref VARCHAR(1024) NOT NULL DEFAULT '',
    expected_revision BIGINT NOT NULL,
    actor_id VARCHAR(512) NOT NULL,
    action VARCHAR(32) NOT NULL,
    result TEXT,
    reason TEXT NOT NULL DEFAULT '',
    applied BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    applied_at DATETIME,
    PRIMARY KEY (tenant_id, run_id, decision_id),
    FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX uq_agent_run_decisions_applied_pending
    ON agent_run_decisions (tenant_id, run_id, pending_id)
    WHERE applied = 1;

CREATE TABLE agent_run_inputs (
    tenant_id INTEGER NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    steer_id VARCHAR(255) NOT NULL,
    mode VARCHAR(32) NOT NULL,
    message TEXT NOT NULL,
    cursor BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at DATETIME,
    PRIMARY KEY (tenant_id, run_id, steer_id),
    FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE
);
