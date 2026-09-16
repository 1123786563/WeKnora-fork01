CREATE TABLE workbench_requests (
    tenant_id INTEGER NOT NULL,
    actor_id VARCHAR(512) NOT NULL,
    request_id VARCHAR(128) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    session_id VARCHAR(64) NOT NULL,
    agent_id VARCHAR(64) NOT NULL DEFAULT '',
    target_id VARCHAR(512) NOT NULL DEFAULT 'platform',
    workspace_ref VARCHAR(512) NOT NULL DEFAULT '',
    text TEXT NOT NULL,
    budget_upper INTEGER NOT NULL DEFAULT 0,
    reservation_ref VARCHAR(512) NOT NULL DEFAULT '',
    state VARCHAR(32) NOT NULL DEFAULT 'pending',
    run_id VARCHAR(64) NOT NULL DEFAULT '',
    reason VARCHAR(512) NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, actor_id, request_id),
    CHECK (state IN ('pending', 'dispatching', 'admitted', 'rejected')),
    CHECK (budget_upper >= 0)
);
CREATE INDEX idx_workbench_requests_lookup ON workbench_requests (tenant_id, actor_id, created_at);
CREATE INDEX idx_workbench_requests_state ON workbench_requests (state, updated_at);
