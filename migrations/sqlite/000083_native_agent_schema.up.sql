-- P1.2 SQLite twin of 000162.  It is a fresh namespace and never copies old
-- Session or Memory rows into native execution state.
CREATE TABLE IF NOT EXISTS native_agent_tenants (
    tenant_id INTEGER PRIMARY KEY REFERENCES tenants(id) ON DELETE RESTRICT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS native_agent_sessions (
    tenant_id INTEGER NOT NULL REFERENCES native_agent_tenants(tenant_id) ON DELETE RESTRICT,
    owner_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id, session_id)
);

CREATE TABLE IF NOT EXISTS native_agent_runs (
    tenant_id INTEGER NOT NULL REFERENCES native_agent_tenants(tenant_id) ON DELETE RESTRICT,
    run_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_epoch INTEGER NOT NULL DEFAULT 0 CHECK (lease_epoch >= 0),
    lease_expires_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id),
    FOREIGN KEY (tenant_id, owner_id, session_id) REFERENCES native_agent_sessions(tenant_id, owner_id, session_id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_native_agent_runs_scope_status ON native_agent_runs (tenant_id, session_id, status, lease_expires_at);

CREATE TABLE IF NOT EXISTS native_agent_inputs (
    tenant_id INTEGER NOT NULL,
    run_id TEXT NOT NULL,
    input_id TEXT NOT NULL,
    input_hash TEXT NOT NULL CHECK (length(input_hash) > 0),
    payload TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, input_id),
    UNIQUE (tenant_id, run_id, input_hash),
    FOREIGN KEY (tenant_id, run_id) REFERENCES native_agent_runs(tenant_id, run_id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS native_agent_config_bindings (
    tenant_id INTEGER NOT NULL,
    run_id TEXT NOT NULL,
    binding_kind TEXT NOT NULL,
    config_ref TEXT NOT NULL,
    config_version INTEGER NOT NULL CHECK (config_version >= 0),
    config_hash TEXT NOT NULL CHECK (length(config_hash) > 0),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, binding_kind),
    FOREIGN KEY (tenant_id, run_id) REFERENCES native_agent_runs(tenant_id, run_id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS native_agent_memory_scopes (
    tenant_id INTEGER NOT NULL REFERENCES native_agent_tenants(tenant_id) ON DELETE RESTRICT,
    user_id TEXT NOT NULL,
    generation INTEGER NOT NULL DEFAULT 0 CHECK (generation >= 0),
    tombstone_generation INTEGER NOT NULL DEFAULT 0 CHECK (tombstone_generation >= 0 AND tombstone_generation <= generation),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    policy_revision INTEGER NOT NULL DEFAULT 0 CHECK (policy_revision >= 0),
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, user_id)
);

CREATE TABLE IF NOT EXISTS native_agent_memory_entries (
    tenant_id INTEGER NOT NULL,
    user_id TEXT NOT NULL,
    memory_id TEXT NOT NULL,
    generation INTEGER NOT NULL CHECK (generation >= 0),
    tombstoned INTEGER NOT NULL DEFAULT 0 CHECK (tombstoned IN (0, 1)),
    content TEXT NOT NULL,
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, user_id, memory_id),
    FOREIGN KEY (tenant_id, user_id) REFERENCES native_agent_memory_scopes(tenant_id, user_id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_native_agent_memory_entries_scope_generation ON native_agent_memory_entries (tenant_id, user_id, generation, tombstoned);

CREATE TABLE IF NOT EXISTS native_agent_attempts (
    tenant_id INTEGER NOT NULL,
    run_id TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    lease_epoch INTEGER NOT NULL CHECK (lease_epoch >= 0),
    status TEXT NOT NULL DEFAULT 'pending',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, attempt_id),
    FOREIGN KEY (tenant_id, run_id) REFERENCES native_agent_runs(tenant_id, run_id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS native_agent_tool_calls (
    tenant_id INTEGER NOT NULL,
    run_id TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    call_id TEXT NOT NULL,
    plan_version INTEGER NOT NULL CHECK (plan_version > 0),
    args_hash TEXT NOT NULL CHECK (length(args_hash) > 0),
    lease_epoch INTEGER NOT NULL CHECK (lease_epoch >= 0),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, attempt_id, call_id),
    UNIQUE (tenant_id, run_id, attempt_id, call_id, plan_version),
    FOREIGN KEY (tenant_id, run_id, attempt_id) REFERENCES native_agent_attempts(tenant_id, run_id, attempt_id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS native_agent_tool_results (
    tenant_id INTEGER NOT NULL,
    run_id TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    call_id TEXT NOT NULL,
    result_hash TEXT NOT NULL CHECK (length(result_hash) > 0),
    outcome TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, attempt_id, call_id),
    FOREIGN KEY (tenant_id, run_id, attempt_id, call_id) REFERENCES native_agent_tool_calls(tenant_id, run_id, attempt_id, call_id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS native_agent_pending_decisions (
    tenant_id INTEGER NOT NULL,
    run_id TEXT NOT NULL,
    pending_id TEXT NOT NULL,
    revision INTEGER NOT NULL DEFAULT 0 CHECK (revision >= 0),
    status TEXT NOT NULL,
    expires_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, pending_id),
    FOREIGN KEY (tenant_id, run_id) REFERENCES native_agent_runs(tenant_id, run_id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_native_agent_pending_scope_status ON native_agent_pending_decisions (tenant_id, run_id, status, created_at);

CREATE TABLE IF NOT EXISTS native_agent_commit_intents (
    tenant_id INTEGER NOT NULL,
    run_id TEXT NOT NULL,
    intent_id TEXT NOT NULL,
    payload_hash TEXT NOT NULL CHECK (length(payload_hash) > 0),
    lease_epoch INTEGER NOT NULL CHECK (lease_epoch >= 0),
    state TEXT NOT NULL DEFAULT 'pending',
    applied_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, intent_id),
    UNIQUE (tenant_id, run_id, payload_hash),
    FOREIGN KEY (tenant_id, run_id) REFERENCES native_agent_runs(tenant_id, run_id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS native_agent_checkpoints (
    tenant_id INTEGER NOT NULL,
    run_id TEXT NOT NULL,
    checkpoint_id TEXT NOT NULL,
    intent_id TEXT NOT NULL,
    lease_epoch INTEGER NOT NULL CHECK (lease_epoch >= 0),
    runnable INTEGER NOT NULL DEFAULT 0 CHECK (runnable IN (0, 1)),
    payload TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, checkpoint_id),
    FOREIGN KEY (tenant_id, run_id, intent_id) REFERENCES native_agent_commit_intents(tenant_id, run_id, intent_id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_native_agent_checkpoints_scope_runnable ON native_agent_checkpoints (tenant_id, run_id, runnable, created_at);

CREATE TABLE IF NOT EXISTS native_agent_session_events (
    tenant_id INTEGER NOT NULL,
    app_name TEXT NOT NULL,
    user_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    stable_event_id TEXT NOT NULL,
    payload_hash TEXT NOT NULL CHECK (length(payload_hash) > 0),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, app_name, user_id, session_id, stable_event_id),
    FOREIGN KEY (tenant_id, user_id, session_id) REFERENCES native_agent_sessions(tenant_id, user_id, session_id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS native_agent_events (
    tenant_id INTEGER NOT NULL,
    run_id TEXT NOT NULL,
    sequence INTEGER NOT NULL CHECK (sequence > 0),
    event_id TEXT NOT NULL,
    intent_id TEXT,
    payload_hash TEXT NOT NULL CHECK (length(payload_hash) > 0),
    payload TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, sequence),
    UNIQUE (tenant_id, run_id, event_id),
    FOREIGN KEY (tenant_id, run_id) REFERENCES native_agent_runs(tenant_id, run_id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, run_id, intent_id) REFERENCES native_agent_commit_intents(tenant_id, run_id, intent_id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS native_agent_usage_observations (
    tenant_id INTEGER NOT NULL,
    run_id TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    observation_id TEXT NOT NULL,
    revision INTEGER NOT NULL DEFAULT 0 CHECK (revision >= 0),
    payload_hash TEXT NOT NULL CHECK (length(payload_hash) > 0),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, attempt_id, observation_id),
    FOREIGN KEY (tenant_id, run_id, attempt_id) REFERENCES native_agent_attempts(tenant_id, run_id, attempt_id) ON DELETE RESTRICT
);

ALTER TABLE native_agent_runs ADD COLUMN request_id TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_runs ADD COLUMN input_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_runs ADD COLUMN revision INTEGER NOT NULL DEFAULT 0 CHECK (revision >= 0);
CREATE UNIQUE INDEX IF NOT EXISTS uq_native_agent_runs_request_scope ON native_agent_runs (tenant_id, owner_id, session_id, request_id);

CREATE TABLE IF NOT EXISTS native_session_state (
    tenant_id INTEGER NOT NULL,
    owner_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    state_key TEXT NOT NULL,
    revision INTEGER NOT NULL DEFAULT 0 CHECK (revision >= 0),
    state_value TEXT NOT NULL DEFAULT '{}',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id, session_id, state_key),
    FOREIGN KEY (tenant_id, owner_id, session_id) REFERENCES native_agent_sessions(tenant_id, owner_id, session_id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_native_session_state_scope_revision ON native_session_state (tenant_id, owner_id, session_id, revision);

CREATE TABLE IF NOT EXISTS native_memory_jobs (
    tenant_id INTEGER NOT NULL,
    subject_id TEXT NOT NULL,
    job_id TEXT NOT NULL,
    generation INTEGER NOT NULL CHECK (generation >= 0),
    through_event_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    policy_revision INTEGER NOT NULL DEFAULT 0 CHECK (policy_revision >= 0),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, subject_id, job_id),
    UNIQUE (tenant_id, subject_id, through_event_id, generation),
    FOREIGN KEY (tenant_id, subject_id) REFERENCES native_agent_memory_scopes(tenant_id, user_id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_native_memory_jobs_scope_status ON native_memory_jobs (tenant_id, subject_id, status, created_at);

ALTER TABLE native_agent_inputs ADD COLUMN role TEXT NOT NULL DEFAULT 'user';
ALTER TABLE native_agent_inputs ADD COLUMN request_id TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_inputs ADD COLUMN revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE native_agent_inputs ADD COLUMN ordinal INTEGER NOT NULL DEFAULT 0;
ALTER TABLE native_agent_inputs ADD COLUMN consumed INTEGER NOT NULL DEFAULT 0 CHECK (consumed IN (0, 1));
CREATE UNIQUE INDEX IF NOT EXISTS uq_native_agent_inputs_request_scope ON native_agent_inputs (tenant_id, run_id, request_id);

ALTER TABLE native_agent_config_bindings ADD COLUMN schema_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE native_agent_config_bindings ADD COLUMN sdk_version TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_config_bindings ADD COLUMN graph_version TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_config_bindings ADD COLUMN credential_ref TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_config_bindings ADD COLUMN credential_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE native_agent_config_bindings ADD COLUMN tool_set_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_config_bindings ADD COLUMN skill_set_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_config_bindings ADD COLUMN execution_target_id TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_config_bindings ADD COLUMN workspace_ref TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_config_bindings ADD COLUMN source TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_config_bindings ADD COLUMN target TEXT NOT NULL DEFAULT '';

ALTER TABLE native_agent_attempts ADD COLUMN kind TEXT NOT NULL DEFAULT 'model';
ALTER TABLE native_agent_attempts ADD COLUMN logical_call_id TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_attempts ADD COLUMN invocation_id TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_attempts ADD COLUMN replaces_attempt_id TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_attempts ADD COLUMN attempt_number INTEGER NOT NULL DEFAULT 0;
ALTER TABLE native_agent_attempts ADD COLUMN provider_request_id TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_attempts ADD COLUMN effect_state TEXT NOT NULL DEFAULT 'not_dispatched';
ALTER TABLE native_agent_attempts ADD COLUMN finished_at DATETIME;
CREATE UNIQUE INDEX IF NOT EXISTS uq_native_agent_attempts_number ON native_agent_attempts (tenant_id, run_id, attempt_number);

CREATE TABLE IF NOT EXISTS native_agent_tool_plans (
    tenant_id INTEGER NOT NULL,
    run_id TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    call_id TEXT NOT NULL,
    plan_version INTEGER NOT NULL CHECK (plan_version > 0),
    provider_tool_call_id TEXT NOT NULL DEFAULT '',
    service_id TEXT NOT NULL DEFAULT '',
    installation_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    schema_hash TEXT NOT NULL DEFAULT '',
    config_version TEXT NOT NULL DEFAULT '',
    args TEXT NOT NULL DEFAULT '{}',
    args_hash TEXT NOT NULL CHECK (length(args_hash) > 0),
    policy TEXT NOT NULL,
    idempotency_key TEXT NOT NULL DEFAULT '',
    idempotency_expires_at DATETIME,
    required_grants TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, attempt_id, call_id, plan_version),
    FOREIGN KEY (tenant_id, run_id, attempt_id, call_id) REFERENCES native_agent_tool_calls(tenant_id, run_id, attempt_id, call_id) ON DELETE RESTRICT
);

ALTER TABLE native_agent_tool_results ADD COLUMN provider_receipt TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_tool_results ADD COLUMN query_anchor TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_tool_results ADD COLUMN effect_state TEXT NOT NULL DEFAULT 'not_dispatched';
ALTER TABLE native_agent_tool_results ADD COLUMN is_error INTEGER NOT NULL DEFAULT 0 CHECK (is_error IN (0, 1));
ALTER TABLE native_agent_tool_results ADD COLUMN truncated INTEGER NOT NULL DEFAULT 0 CHECK (truncated IN (0, 1));
ALTER TABLE native_agent_tool_results ADD COLUMN content TEXT NOT NULL DEFAULT '{}';
ALTER TABLE native_agent_tool_results ADD COLUMN failure TEXT;

ALTER TABLE native_agent_pending_decisions ADD COLUMN call_id TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_pending_decisions ADD COLUMN plan_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE native_agent_pending_decisions ADD COLUMN args_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_pending_decisions ADD COLUMN wait_kind TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_pending_decisions ADD COLUMN expected_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE native_agent_pending_decisions ADD COLUMN decision_id TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_pending_decisions ADD COLUMN action TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_pending_decisions ADD COLUMN resource_ref TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS uq_native_agent_pending_decision_id ON native_agent_pending_decisions (tenant_id, run_id, decision_id) WHERE decision_id <> '';

ALTER TABLE native_agent_commit_intents ADD COLUMN version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE native_agent_commit_intents ADD COLUMN payload TEXT NOT NULL DEFAULT '{}';
ALTER TABLE native_agent_commit_intents ADD COLUMN terminal_status TEXT NOT NULL DEFAULT '';

ALTER TABLE native_agent_checkpoints ADD COLUMN schema_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE native_agent_checkpoints ADD COLUMN sdk_version TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_checkpoints ADD COLUMN graph_version TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_checkpoints ADD COLUMN namespace TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_checkpoints ADD COLUMN lineage_id TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_checkpoints ADD COLUMN request_payload TEXT NOT NULL DEFAULT '{}';
ALTER TABLE native_agent_checkpoints ADD COLUMN result_call_ids TEXT NOT NULL DEFAULT '[]';

ALTER TABLE native_agent_session_events ADD COLUMN payload TEXT NOT NULL DEFAULT '{}';
ALTER TABLE native_agent_session_events ADD COLUMN ordinal INTEGER NOT NULL DEFAULT 0 CHECK (ordinal >= 0);
CREATE UNIQUE INDEX IF NOT EXISTS uq_native_agent_session_events_ordinal ON native_agent_session_events (tenant_id, app_name, user_id, session_id, ordinal);

ALTER TABLE native_agent_usage_observations ADD COLUMN provider TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_usage_observations ADD COLUMN model TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_usage_observations ADD COLUMN input_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE native_agent_usage_observations ADD COLUMN output_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE native_agent_usage_observations ADD COLUMN cached_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE native_agent_usage_observations ADD COLUMN cost_micros INTEGER NOT NULL DEFAULT 0;
ALTER TABLE native_agent_usage_observations ADD COLUMN provider_request_id TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_usage_observations ADD COLUMN funding_ref TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_usage_observations ADD COLUMN budget_root_run_id TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_usage_observations ADD COLUMN cache_read_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE native_agent_usage_observations ADD COLUMN cache_create_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE native_agent_usage_observations ADD COLUMN accounting_status TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_usage_observations ADD COLUMN dimensions TEXT NOT NULL DEFAULT '{}';
ALTER TABLE native_agent_usage_observations ADD COLUMN occurred_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP;

ALTER TABLE native_agent_events ADD COLUMN protocol TEXT NOT NULL DEFAULT 'weknora.agent.v1';
ALTER TABLE native_agent_events ADD COLUMN schema_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE native_agent_events ADD COLUMN attempt_id TEXT NOT NULL DEFAULT '';
ALTER TABLE native_agent_events ADD COLUMN kind TEXT NOT NULL DEFAULT '';
