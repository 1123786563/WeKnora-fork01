CREATE TABLE workbench_interactions (
    tenant_id BIGINT NOT NULL,
    id VARCHAR(128) NOT NULL,
    run_id VARCHAR(128) NOT NULL,
    owner_id VARCHAR(255) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    args_hash VARCHAR(128) NOT NULL DEFAULT '',
    decision_id VARCHAR(128) NOT NULL DEFAULT '',
    action VARCHAR(32) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    expected_revision BIGINT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, id),
    CONSTRAINT ck_workbench_interaction_kind CHECK (kind IN ('tool_approval','budget','recovery')),
    CONSTRAINT ck_workbench_interaction_action CHECK (action = '' OR (kind = 'tool_approval' AND action IN ('approve','reject')) OR (kind = 'budget' AND action = 'extend') OR (kind = 'recovery' AND action IN ('retry','provide_result','terminate'))),
    CONSTRAINT fk_workbench_interaction_run FOREIGN KEY (tenant_id, run_id) REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE
);
CREATE INDEX idx_workbench_interactions_owner ON workbench_interactions (tenant_id, owner_id, run_id, created_at);
CREATE UNIQUE INDEX uq_workbench_interactions_decision ON workbench_interactions (tenant_id, id, decision_id) WHERE decision_id <> '';
