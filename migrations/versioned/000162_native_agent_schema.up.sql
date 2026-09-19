-- P1.2: isolated durable storage for the native agent path.  These tables do
-- not import or mutate legacy Session/Memory records.
CREATE TABLE IF NOT EXISTS native_agent_tenants (
    tenant_id BIGINT PRIMARY KEY REFERENCES tenants(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS native_agent_sessions (
    tenant_id BIGINT NOT NULL REFERENCES native_agent_tenants(tenant_id) ON DELETE RESTRICT,
    user_id VARCHAR(255) NOT NULL,
    session_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, user_id, session_id),
    UNIQUE (tenant_id, session_id)
);

CREATE TABLE IF NOT EXISTS native_agent_runs (
    tenant_id BIGINT NOT NULL REFERENCES native_agent_tenants(tenant_id) ON DELETE RESTRICT,
    run_id VARCHAR(255) NOT NULL,
    session_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    status VARCHAR(64) NOT NULL DEFAULT 'pending',
    lease_owner VARCHAR(255) NOT NULL DEFAULT '',
    lease_epoch BIGINT NOT NULL DEFAULT 0 CHECK (lease_epoch >= 0),
    lease_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id),
    FOREIGN KEY (tenant_id, user_id, session_id) REFERENCES native_agent_sessions(tenant_id, user_id, session_id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_native_agent_runs_scope_status ON native_agent_runs (tenant_id, session_id, status, lease_expires_at);

CREATE TABLE IF NOT EXISTS native_agent_inputs (
    tenant_id BIGINT NOT NULL,
    run_id VARCHAR(255) NOT NULL,
    input_id VARCHAR(255) NOT NULL,
    input_hash VARCHAR(128) NOT NULL CHECK (length(input_hash) > 0),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, input_id),
    UNIQUE (tenant_id, run_id, input_hash),
    FOREIGN KEY (tenant_id, run_id) REFERENCES native_agent_runs(tenant_id, run_id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS native_agent_config_bindings (
    tenant_id BIGINT NOT NULL,
    run_id VARCHAR(255) NOT NULL,
    binding_kind VARCHAR(64) NOT NULL,
    config_ref VARCHAR(255) NOT NULL,
    config_version BIGINT NOT NULL CHECK (config_version >= 0),
    config_hash VARCHAR(128) NOT NULL CHECK (length(config_hash) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, binding_kind),
    FOREIGN KEY (tenant_id, run_id) REFERENCES native_agent_runs(tenant_id, run_id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS native_agent_memory_scopes (
    tenant_id BIGINT NOT NULL REFERENCES native_agent_tenants(tenant_id) ON DELETE RESTRICT,
    user_id VARCHAR(255) NOT NULL,
    generation BIGINT NOT NULL DEFAULT 0 CHECK (generation >= 0),
    tombstone_generation BIGINT NOT NULL DEFAULT 0 CHECK (tombstone_generation >= 0 AND tombstone_generation <= generation),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    policy_revision BIGINT NOT NULL DEFAULT 0 CHECK (policy_revision >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, user_id)
);

CREATE TABLE IF NOT EXISTS native_agent_memory_entries (
    tenant_id BIGINT NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    memory_id VARCHAR(255) NOT NULL,
    generation BIGINT NOT NULL CHECK (generation >= 0),
    tombstoned BOOLEAN NOT NULL DEFAULT FALSE,
    content TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, user_id, memory_id),
    FOREIGN KEY (tenant_id, user_id) REFERENCES native_agent_memory_scopes(tenant_id, user_id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_native_agent_memory_entries_scope_generation ON native_agent_memory_entries (tenant_id, user_id, generation, tombstoned);

CREATE TABLE IF NOT EXISTS native_agent_attempts (
    tenant_id BIGINT NOT NULL,
    run_id VARCHAR(255) NOT NULL,
    attempt_id VARCHAR(255) NOT NULL,
    lease_epoch BIGINT NOT NULL CHECK (lease_epoch >= 0),
    status VARCHAR(64) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, attempt_id),
    FOREIGN KEY (tenant_id, run_id) REFERENCES native_agent_runs(tenant_id, run_id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS native_agent_tool_calls (
    tenant_id BIGINT NOT NULL,
    run_id VARCHAR(255) NOT NULL,
    attempt_id VARCHAR(255) NOT NULL,
    call_id VARCHAR(255) NOT NULL,
    plan_version BIGINT NOT NULL CHECK (plan_version > 0),
    args_hash VARCHAR(128) NOT NULL CHECK (length(args_hash) > 0),
    lease_epoch BIGINT NOT NULL CHECK (lease_epoch >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, attempt_id, call_id),
    UNIQUE (tenant_id, run_id, attempt_id, call_id, plan_version),
    FOREIGN KEY (tenant_id, run_id, attempt_id) REFERENCES native_agent_attempts(tenant_id, run_id, attempt_id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS native_agent_tool_results (
    tenant_id BIGINT NOT NULL,
    run_id VARCHAR(255) NOT NULL,
    attempt_id VARCHAR(255) NOT NULL,
    call_id VARCHAR(255) NOT NULL,
    result_hash VARCHAR(128) NOT NULL CHECK (length(result_hash) > 0),
    outcome JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, attempt_id, call_id),
    FOREIGN KEY (tenant_id, run_id, attempt_id, call_id) REFERENCES native_agent_tool_calls(tenant_id, run_id, attempt_id, call_id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS native_agent_pending_decisions (
    tenant_id BIGINT NOT NULL,
    run_id VARCHAR(255) NOT NULL,
    pending_id VARCHAR(255) NOT NULL,
    revision BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0),
    status VARCHAR(64) NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, pending_id),
    FOREIGN KEY (tenant_id, run_id) REFERENCES native_agent_runs(tenant_id, run_id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_native_agent_pending_scope_status ON native_agent_pending_decisions (tenant_id, run_id, status, created_at);

CREATE TABLE IF NOT EXISTS native_agent_commit_intents (
    tenant_id BIGINT NOT NULL,
    run_id VARCHAR(255) NOT NULL,
    intent_id VARCHAR(255) NOT NULL,
    payload_hash VARCHAR(128) NOT NULL CHECK (length(payload_hash) > 0),
    lease_epoch BIGINT NOT NULL CHECK (lease_epoch >= 0),
    state VARCHAR(32) NOT NULL DEFAULT 'pending',
    applied_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, intent_id),
    UNIQUE (tenant_id, run_id, payload_hash),
    FOREIGN KEY (tenant_id, run_id) REFERENCES native_agent_runs(tenant_id, run_id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS native_agent_checkpoints (
    tenant_id BIGINT NOT NULL,
    run_id VARCHAR(255) NOT NULL,
    checkpoint_id VARCHAR(255) NOT NULL,
    intent_id VARCHAR(255) NOT NULL,
    lease_epoch BIGINT NOT NULL CHECK (lease_epoch >= 0),
    runnable BOOLEAN NOT NULL DEFAULT FALSE,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, checkpoint_id),
    FOREIGN KEY (tenant_id, run_id, intent_id) REFERENCES native_agent_commit_intents(tenant_id, run_id, intent_id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_native_agent_checkpoints_scope_runnable ON native_agent_checkpoints (tenant_id, run_id, runnable, created_at);

CREATE TABLE IF NOT EXISTS native_agent_session_events (
    tenant_id BIGINT NOT NULL,
    app_name VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    session_id VARCHAR(255) NOT NULL,
    stable_event_id VARCHAR(255) NOT NULL,
    payload_hash VARCHAR(128) NOT NULL CHECK (length(payload_hash) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, app_name, user_id, session_id, stable_event_id),
    FOREIGN KEY (tenant_id, user_id, session_id) REFERENCES native_agent_sessions(tenant_id, user_id, session_id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS native_agent_events (
    tenant_id BIGINT NOT NULL,
    run_id VARCHAR(255) NOT NULL,
    sequence BIGINT NOT NULL CHECK (sequence > 0),
    event_id VARCHAR(255) NOT NULL,
    intent_id VARCHAR(255),
    payload_hash VARCHAR(128) NOT NULL CHECK (length(payload_hash) > 0),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, sequence),
    UNIQUE (tenant_id, run_id, event_id),
    FOREIGN KEY (tenant_id, run_id) REFERENCES native_agent_runs(tenant_id, run_id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, run_id, intent_id) REFERENCES native_agent_commit_intents(tenant_id, run_id, intent_id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS native_agent_usage_observations (
    tenant_id BIGINT NOT NULL,
    run_id VARCHAR(255) NOT NULL,
    attempt_id VARCHAR(255) NOT NULL,
    observation_id VARCHAR(255) NOT NULL,
    revision BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0),
    payload_hash VARCHAR(128) NOT NULL CHECK (length(payload_hash) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, attempt_id, observation_id),
    FOREIGN KEY (tenant_id, run_id, attempt_id) REFERENCES native_agent_attempts(tenant_id, run_id, attempt_id) ON DELETE RESTRICT
);
