-- T16 (#134): the durable Workspace writer lease. One row per Workspace is
-- the compare-and-swap identity: two concurrent writers race a single
-- insert/update on workspace_id and the database decides the winner. The row
-- binds the Task (session), the Workspace, the one writing Run and the
-- draft-head revision fenced at acquisition. The lease does NOT foreign-key
-- agent_runs: it must outlive an unobservable run row (an unknown outcome
-- retains the fence), and the store re-verifies the authoritative run state
-- inside every release transaction instead.
CREATE TABLE craft_workspace_writer_leases (
    workspace_id TEXT PRIMARY KEY NOT NULL,
    tenant_id BIGINT NOT NULL,
    session_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    revision BIGINT NOT NULL CHECK (revision >= 0),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, run_id),
    CHECK (session_id <> '' AND run_id <> ''),
    FOREIGN KEY (tenant_id, workspace_id) REFERENCES craft_workspaces(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, session_id) REFERENCES sessions(tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_craft_writer_leases_session ON craft_workspace_writer_leases (tenant_id, session_id);
