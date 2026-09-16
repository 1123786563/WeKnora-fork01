-- This migration owns its transaction. The SQLite migrator must use
-- NoTxWrap: PRAGMA foreign_keys only changes outside a transaction.
PRAGMA foreign_keys = OFF;
BEGIN IMMEDIATE;

CREATE TABLE agent_runs_rebuilt (
    tenant_id INTEGER NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    request_id VARCHAR(64) NOT NULL,
    assistant_message_id VARCHAR(36) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    engine_type VARCHAR(16) NOT NULL DEFAULT 'trpc',
    driver VARCHAR(16) NOT NULL DEFAULT 'platform',
    target_id VARCHAR(512) NOT NULL DEFAULT '',
    budget_ref VARCHAR(512) NOT NULL DEFAULT '',
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
    CHECK (
        (driver = 'platform' AND engine_type = 'trpc')
        OR (driver = 'paseo' AND engine_type = '')
    ),
    FOREIGN KEY (tenant_id, session_id)
        REFERENCES sessions (tenant_id, id) ON DELETE CASCADE
);

INSERT INTO agent_runs_rebuilt (
    tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id,
    request_hash, engine_type, driver, target_id, budget_ref, status, wait_reason,
    snapshot, graph_version, sdk_version, schema_version, lease_owner, lease_until,
    epoch, revision, max_rounds, max_tool_calls, token_budget, deadline, created_at, updated_at
)
SELECT
    tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id,
    request_hash, engine_type, 'platform', '', '', status, wait_reason,
    snapshot, graph_version, sdk_version, schema_version, lease_owner, lease_until,
    epoch, revision, max_rounds, max_tool_calls, token_budget, deadline, created_at, updated_at
FROM agent_runs;

DROP TABLE agent_runs;
ALTER TABLE agent_runs_rebuilt RENAME TO agent_runs;

CREATE UNIQUE INDEX uq_agent_runs_request
    ON agent_runs (tenant_id, owner_id, request_id);
CREATE INDEX idx_agent_runs_recovery_scan
    ON agent_runs (status, lease_until, created_at);
CREATE INDEX idx_agent_runs_session
    ON agent_runs (tenant_id, session_id);
CREATE INDEX idx_agent_runs_driver_scan
    ON agent_runs (driver, status, lease_until, created_at);

CREATE TEMP TABLE workbench_fk_guard (ok INTEGER NOT NULL CHECK (ok = 1));
INSERT INTO workbench_fk_guard
SELECT CASE WHEN EXISTS (SELECT 1 FROM pragma_foreign_key_check) THEN 0 ELSE 1 END;
DROP TABLE workbench_fk_guard;

COMMIT;
PRAGMA foreign_keys = ON;
