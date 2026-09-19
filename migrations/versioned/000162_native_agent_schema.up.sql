-- P1.2: isolated durable storage for the native agent path.  These tables do
-- not import or mutate legacy Session/Memory records.
CREATE TABLE IF NOT EXISTS native_agent_tenants (
    tenant_id BIGINT PRIMARY KEY REFERENCES tenants(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS native_agent_sessions (
    tenant_id BIGINT NOT NULL REFERENCES native_agent_tenants(tenant_id) ON DELETE RESTRICT,
    owner_id VARCHAR(255) NOT NULL,
    session_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id, session_id)
);

CREATE TABLE IF NOT EXISTS native_agent_runs (
    tenant_id BIGINT NOT NULL REFERENCES native_agent_tenants(tenant_id) ON DELETE RESTRICT,
    run_id VARCHAR(255) NOT NULL,
    session_id VARCHAR(255) NOT NULL,
    owner_id VARCHAR(255) NOT NULL,
    status VARCHAR(64) NOT NULL DEFAULT 'pending',
    lease_owner VARCHAR(255) NOT NULL DEFAULT '',
    lease_epoch BIGINT NOT NULL DEFAULT 0 CHECK (lease_epoch >= 0),
    lease_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id),
    FOREIGN KEY (tenant_id, owner_id, session_id) REFERENCES native_agent_sessions(tenant_id, owner_id, session_id) ON DELETE RESTRICT
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

-- The remaining fields are additive inside this new migration so both active
-- dialects start with the complete P1.2 contract rather than relying on a
-- later repository-side schema patch.
ALTER TABLE native_agent_runs ADD COLUMN IF NOT EXISTS request_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_runs ADD COLUMN IF NOT EXISTS input_hash VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE native_agent_runs ADD COLUMN IF NOT EXISTS revision BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0);
CREATE UNIQUE INDEX IF NOT EXISTS uq_native_agent_runs_request_scope ON native_agent_runs (tenant_id, owner_id, session_id, request_id);

CREATE TABLE IF NOT EXISTS native_session_state (
    tenant_id BIGINT NOT NULL,
    owner_id VARCHAR(255) NOT NULL,
    session_id VARCHAR(255) NOT NULL,
    state_key VARCHAR(255) NOT NULL,
    revision BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0),
    state_value JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id, session_id, state_key),
    FOREIGN KEY (tenant_id, owner_id, session_id) REFERENCES native_agent_sessions(tenant_id, owner_id, session_id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_native_session_state_scope_revision ON native_session_state (tenant_id, owner_id, session_id, revision);

CREATE TABLE IF NOT EXISTS native_memory_jobs (
    tenant_id BIGINT NOT NULL,
    subject_id VARCHAR(255) NOT NULL,
    job_id VARCHAR(255) NOT NULL,
    generation BIGINT NOT NULL CHECK (generation >= 0),
    through_event_id VARCHAR(255) NOT NULL,
    status VARCHAR(64) NOT NULL DEFAULT 'queued',
    policy_revision BIGINT NOT NULL DEFAULT 0 CHECK (policy_revision >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, subject_id, job_id),
    UNIQUE (tenant_id, subject_id, through_event_id, generation),
    FOREIGN KEY (tenant_id, subject_id) REFERENCES native_agent_memory_scopes(tenant_id, user_id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_native_memory_jobs_scope_status ON native_memory_jobs (tenant_id, subject_id, status, created_at);

ALTER TABLE native_agent_inputs ADD COLUMN IF NOT EXISTS role VARCHAR(64) NOT NULL DEFAULT 'user';
ALTER TABLE native_agent_inputs ADD COLUMN IF NOT EXISTS request_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_inputs ADD COLUMN IF NOT EXISTS revision BIGINT NOT NULL DEFAULT 0;
ALTER TABLE native_agent_inputs ADD COLUMN IF NOT EXISTS ordinal BIGINT NOT NULL DEFAULT 0;
ALTER TABLE native_agent_inputs ADD COLUMN IF NOT EXISTS consumed BOOLEAN NOT NULL DEFAULT FALSE;
CREATE UNIQUE INDEX IF NOT EXISTS uq_native_agent_inputs_request_scope ON native_agent_inputs (tenant_id, run_id, request_id);

ALTER TABLE native_agent_config_bindings ADD COLUMN IF NOT EXISTS schema_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE native_agent_config_bindings ADD COLUMN IF NOT EXISTS sdk_version VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_config_bindings ADD COLUMN IF NOT EXISTS graph_version VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_config_bindings ADD COLUMN IF NOT EXISTS credential_ref VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_config_bindings ADD COLUMN IF NOT EXISTS credential_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE native_agent_config_bindings ADD COLUMN IF NOT EXISTS tool_set_hash VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE native_agent_config_bindings ADD COLUMN IF NOT EXISTS skill_set_hash VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE native_agent_config_bindings ADD COLUMN IF NOT EXISTS execution_target_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_config_bindings ADD COLUMN IF NOT EXISTS workspace_ref VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_config_bindings ADD COLUMN IF NOT EXISTS source VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_config_bindings ADD COLUMN IF NOT EXISTS target VARCHAR(255) NOT NULL DEFAULT '';

ALTER TABLE native_agent_attempts ADD COLUMN IF NOT EXISTS kind VARCHAR(32) NOT NULL DEFAULT 'model';
ALTER TABLE native_agent_attempts ADD COLUMN IF NOT EXISTS logical_call_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_attempts ADD COLUMN IF NOT EXISTS invocation_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_attempts ADD COLUMN IF NOT EXISTS replaces_attempt_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_attempts ADD COLUMN IF NOT EXISTS attempt_number BIGINT NOT NULL DEFAULT 0;
ALTER TABLE native_agent_attempts ADD COLUMN IF NOT EXISTS provider_request_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_attempts ADD COLUMN IF NOT EXISTS effect_state VARCHAR(32) NOT NULL DEFAULT 'not_dispatched';
ALTER TABLE native_agent_attempts ADD COLUMN IF NOT EXISTS finished_at TIMESTAMPTZ;
CREATE UNIQUE INDEX IF NOT EXISTS uq_native_agent_attempts_number ON native_agent_attempts (tenant_id, run_id, attempt_number);

CREATE TABLE IF NOT EXISTS native_agent_tool_plans (
    tenant_id BIGINT NOT NULL,
    run_id VARCHAR(255) NOT NULL,
    attempt_id VARCHAR(255) NOT NULL,
    call_id VARCHAR(255) NOT NULL,
    plan_version BIGINT NOT NULL CHECK (plan_version > 0),
    provider_tool_call_id VARCHAR(255) NOT NULL DEFAULT '',
    service_id VARCHAR(255) NOT NULL DEFAULT '',
    installation_id VARCHAR(255) NOT NULL DEFAULT '',
    name VARCHAR(255) NOT NULL DEFAULT '',
    schema_hash VARCHAR(128) NOT NULL DEFAULT '',
    config_version VARCHAR(255) NOT NULL DEFAULT '',
    args JSONB NOT NULL DEFAULT '{}'::jsonb,
    args_hash VARCHAR(128) NOT NULL CHECK (length(args_hash) > 0),
    policy VARCHAR(32) NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL DEFAULT '',
    idempotency_expires_at TIMESTAMPTZ,
    required_grants JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, attempt_id, call_id, plan_version),
    FOREIGN KEY (tenant_id, run_id, attempt_id, call_id) REFERENCES native_agent_tool_calls(tenant_id, run_id, attempt_id, call_id) ON DELETE RESTRICT
);

ALTER TABLE native_agent_tool_results ADD COLUMN IF NOT EXISTS provider_receipt TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_tool_results ADD COLUMN IF NOT EXISTS query_anchor TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_tool_results ADD COLUMN IF NOT EXISTS effect_state VARCHAR(32) NOT NULL DEFAULT 'not_dispatched';
ALTER TABLE native_agent_tool_results ADD COLUMN IF NOT EXISTS is_error BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE native_agent_tool_results ADD COLUMN IF NOT EXISTS truncated BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE native_agent_tool_results ADD COLUMN IF NOT EXISTS content JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE native_agent_tool_results ADD COLUMN IF NOT EXISTS failure JSONB;

ALTER TABLE native_agent_pending_decisions ADD COLUMN IF NOT EXISTS call_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_pending_decisions ADD COLUMN IF NOT EXISTS plan_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE native_agent_pending_decisions ADD COLUMN IF NOT EXISTS args_hash VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE native_agent_pending_decisions ADD COLUMN IF NOT EXISTS wait_kind VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE native_agent_pending_decisions ADD COLUMN IF NOT EXISTS expected_revision BIGINT NOT NULL DEFAULT 0;
ALTER TABLE native_agent_pending_decisions ADD COLUMN IF NOT EXISTS decision_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_pending_decisions ADD COLUMN IF NOT EXISTS action VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE native_agent_pending_decisions ADD COLUMN IF NOT EXISTS resource_ref VARCHAR(255) NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS uq_native_agent_pending_decision_id ON native_agent_pending_decisions (tenant_id, run_id, decision_id) WHERE decision_id <> '';

ALTER TABLE native_agent_commit_intents ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE native_agent_commit_intents ADD COLUMN IF NOT EXISTS payload JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE native_agent_commit_intents ADD COLUMN IF NOT EXISTS terminal_status VARCHAR(64) NOT NULL DEFAULT '';

ALTER TABLE native_agent_checkpoints ADD COLUMN IF NOT EXISTS schema_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE native_agent_checkpoints ADD COLUMN IF NOT EXISTS sdk_version VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_checkpoints ADD COLUMN IF NOT EXISTS graph_version VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_checkpoints ADD COLUMN IF NOT EXISTS namespace VARCHAR(512) NOT NULL DEFAULT '';
ALTER TABLE native_agent_checkpoints ADD COLUMN IF NOT EXISTS lineage_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_checkpoints ADD COLUMN IF NOT EXISTS request_payload JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE native_agent_checkpoints ADD COLUMN IF NOT EXISTS result_call_ids JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE native_agent_session_events ADD COLUMN IF NOT EXISTS payload JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE native_agent_session_events ADD COLUMN IF NOT EXISTS ordinal BIGINT NOT NULL DEFAULT 0 CHECK (ordinal >= 0);
CREATE UNIQUE INDEX IF NOT EXISTS uq_native_agent_session_events_ordinal ON native_agent_session_events (tenant_id, app_name, user_id, session_id, ordinal);

ALTER TABLE native_agent_usage_observations ADD COLUMN IF NOT EXISTS provider VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_usage_observations ADD COLUMN IF NOT EXISTS model VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_usage_observations ADD COLUMN IF NOT EXISTS input_tokens BIGINT NOT NULL DEFAULT 0;
ALTER TABLE native_agent_usage_observations ADD COLUMN IF NOT EXISTS output_tokens BIGINT NOT NULL DEFAULT 0;
ALTER TABLE native_agent_usage_observations ADD COLUMN IF NOT EXISTS cached_tokens BIGINT NOT NULL DEFAULT 0;
ALTER TABLE native_agent_usage_observations ADD COLUMN IF NOT EXISTS cost_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE native_agent_usage_observations ADD COLUMN IF NOT EXISTS provider_request_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_usage_observations ADD COLUMN IF NOT EXISTS funding_ref VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_usage_observations ADD COLUMN IF NOT EXISTS budget_root_run_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_usage_observations ADD COLUMN IF NOT EXISTS cache_read_tokens BIGINT NOT NULL DEFAULT 0;
ALTER TABLE native_agent_usage_observations ADD COLUMN IF NOT EXISTS cache_create_tokens BIGINT NOT NULL DEFAULT 0;
ALTER TABLE native_agent_usage_observations ADD COLUMN IF NOT EXISTS accounting_status VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE native_agent_usage_observations ADD COLUMN IF NOT EXISTS dimensions JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE native_agent_usage_observations ADD COLUMN IF NOT EXISTS occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;

ALTER TABLE native_agent_events ADD COLUMN IF NOT EXISTS protocol VARCHAR(64) NOT NULL DEFAULT 'weknora.agent.v1';
ALTER TABLE native_agent_events ADD COLUMN IF NOT EXISTS schema_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE native_agent_events ADD COLUMN IF NOT EXISTS attempt_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE native_agent_events ADD COLUMN IF NOT EXISTS kind VARCHAR(128) NOT NULL DEFAULT '';
