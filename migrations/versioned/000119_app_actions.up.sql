CREATE TABLE app_actions (
    id TEXT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    actor_id TEXT NOT NULL DEFAULT '',
    connection_id TEXT NOT NULL,
    app_version TEXT NOT NULL,
    target TEXT NOT NULL,
    risk TEXT NOT NULL,
    auth_version BIGINT NOT NULL DEFAULT 1,
    args_snapshot TEXT NOT NULL,
    args_digest TEXT NOT NULL,
    state TEXT NOT NULL,
    provider_key TEXT NOT NULL DEFAULT '',
    provider_result TEXT NOT NULL DEFAULT '',
    reservation_id TEXT NOT NULL DEFAULT '',
    fence BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_app_actions_tenant_digest ON app_actions (tenant_id, args_digest);
CREATE INDEX idx_app_actions_state ON app_actions (state);

CREATE TABLE app_action_approvals (
    args_digest TEXT PRIMARY KEY,
    action_id TEXT NOT NULL,
    actor TEXT NOT NULL,
    expiry TIMESTAMP NOT NULL,
    remaining BIGINT NOT NULL DEFAULT 1
);

CREATE TABLE app_action_preauthorizations (
    id TEXT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    allowed_risks_json TEXT NOT NULL,
    connection_id TEXT NOT NULL DEFAULT '*',
    target_scope TEXT NOT NULL DEFAULT '*',
    valid_from TIMESTAMP NOT NULL,
    valid_until TIMESTAMP NOT NULL,
    budget_cap_micro BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_app_action_preauth_tenant ON app_action_preauthorizations (tenant_id);
