CREATE TABLE workbench_interactions (
    tenant_id INTEGER NOT NULL,
    id VARCHAR(128) NOT NULL,
    run_id VARCHAR(128) NOT NULL,
    owner_id VARCHAR(255) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    args_hash VARCHAR(128) NOT NULL DEFAULT '',
    decision_id VARCHAR(128) NOT NULL DEFAULT '',
    action VARCHAR(32) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    expected_revision INTEGER NOT NULL DEFAULT 0,
    expires_at DATETIME,
    revoked INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, id),
    CHECK (kind IN ('tool_approval','budget','recovery')),
    CHECK (length(trim(args_hash)) > 0),
    CHECK (action = '' OR (kind = 'tool_approval' AND action IN ('approve','reject')) OR (kind = 'budget' AND action = 'extend') OR (kind = 'recovery' AND action IN ('retry','provide_result','terminate'))),
    CHECK (length(trim(run_id)) > 0),
    FOREIGN KEY (tenant_id, run_id) REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE
);
CREATE INDEX idx_workbench_interactions_owner ON workbench_interactions (tenant_id, owner_id, run_id, created_at);
CREATE UNIQUE INDEX uq_workbench_interactions_decision ON workbench_interactions (tenant_id, id, decision_id);
