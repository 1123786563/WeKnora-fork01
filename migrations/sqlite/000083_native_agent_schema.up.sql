-- P1.2 SQLite twin of 000162.  It is a fresh namespace and never copies old
-- Session or Memory rows into native execution state.
CREATE TABLE IF NOT EXISTS native_agent_tenants (
    tenant_id INTEGER PRIMARY KEY REFERENCES tenants(id) ON DELETE RESTRICT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS native_agent_sessions (
    tenant_id INTEGER NOT NULL REFERENCES native_agent_tenants(tenant_id) ON DELETE RESTRICT,
    user_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, user_id, session_id),
    UNIQUE (tenant_id, session_id)
);

CREATE TABLE IF NOT EXISTS native_agent_runs (
    tenant_id INTEGER NOT NULL REFERENCES native_agent_tenants(tenant_id) ON DELETE RESTRICT,
    run_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_epoch INTEGER NOT NULL DEFAULT 0 CHECK (lease_epoch >= 0),
    lease_expires_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id),
    FOREIGN KEY (tenant_id, user_id, session_id) REFERENCES native_agent_sessions(tenant_id, user_id, session_id) ON DELETE RESTRICT
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
