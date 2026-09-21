-- SP3: Craft scheduled tasks (C-23), SQLite dialect of versioned
-- 000178_craft_scheduled (same logical constraints).
--
-- craft_scheduled_tasks is ONE user-owned recipe. cron_expression is the
-- single source of truth (the three editor modes compile into it at save
-- time; editor_mode stays only as a UI hint). next_run_at is the
-- dispatcher's claim ticket: a due row is claimed by a compare-and-swap
-- UPDATE keyed on the old next_run_at value (Ruling P-1 — the portable
-- equivalent of SELECT ... FOR UPDATE SKIP LOCKED), so at most one instance
-- fires a tick. Pausing sets next_run_at NULL; deletion is soft.
CREATE TABLE craft_scheduled_tasks (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    name VARCHAR(128) NOT NULL,
    prompt TEXT NOT NULL,
    cron_expression VARCHAR(64) NOT NULL,
    editor_mode VARCHAR(16) NOT NULL DEFAULT 'advanced',
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    next_run_at DATETIME NULL,
    last_run_at DATETIME NULL,
    deleted_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- The dispatcher's due scan: status = 'active' AND deleted_at IS NULL AND
-- next_run_at <= now, oldest first, batched.
CREATE INDEX ix_craft_scheduled_dispatch
    ON craft_scheduled_tasks (status, deleted_at, next_run_at);
-- The owner's task list and the per-owner 404 scoping.
CREATE INDEX ix_craft_scheduled_tasks_owner
    ON craft_scheduled_tasks (tenant_id, owner_id, deleted_at);

-- craft_scheduled_task_runs is the append-only fire ledger of one task.
-- task_id physically cascades with the task; session_id stays a LOGICAL
-- link with SET NULL semantics — no physical FK, the craft_lifecycle
-- precedent: the run history remains readable after the session row is
-- gone.
CREATE TABLE craft_scheduled_task_runs (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    task_id VARCHAR(36) NOT NULL,
    session_id VARCHAR(36) NULL,
    status VARCHAR(24) NOT NULL,
    trigger_source VARCHAR(16) NOT NULL,
    skip_reason VARCHAR(32) NULL,
    error_class VARCHAR(32) NULL,
    error_detail TEXT NULL,
    started_at DATETIME NULL,
    finished_at DATETIME NULL,
    summary TEXT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES craft_scheduled_tasks (id) ON DELETE CASCADE
);

-- The run-history keyset page: WHERE task_id = ? AND started_at < ?
-- ORDER BY started_at DESC.
CREATE INDEX ix_craft_scheduled_task_runs_history
    ON craft_scheduled_task_runs (task_id, started_at);
