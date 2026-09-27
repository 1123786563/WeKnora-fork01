-- T16 (#134): the durable Workspace writer lease. One row per Workspace is
-- the compare-and-swap identity: two concurrent writers race a single
-- insert/update on workspace_id and the database decides the winner. The row
-- binds the Task (session), the Workspace, the one writing Run and the
-- draft-head revision fenced at acquisition. The lease does NOT foreign-key
-- agent_runs: it must outlive an unobservable run row (an unknown outcome
-- retains the fence), and the store re-verifies the authoritative run state
-- inside every release transaction instead.
CREATE TABLE craft_workspace_writer_leases (
    workspace_id VARCHAR(64) NOT NULL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    revision BIGINT NOT NULL CHECK (revision >= 0),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_craft_writer_leases_run UNIQUE (tenant_id, run_id),
    CONSTRAINT ck_craft_writer_leases_identity CHECK (session_id <> '' AND run_id <> ''),
    CONSTRAINT fk_craft_writer_leases_workspace
        FOREIGN KEY (workspace_id)
        REFERENCES craft_workspaces (id) ON DELETE CASCADE,
    CONSTRAINT fk_craft_writer_leases_session
        FOREIGN KEY (session_id)
        REFERENCES sessions (id) ON DELETE CASCADE
);

CREATE INDEX idx_craft_writer_leases_session ON craft_workspace_writer_leases (tenant_id, session_id);
