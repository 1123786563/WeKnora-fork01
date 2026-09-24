# RunView Retention Fix Report

Status: **Run and Session deletion now fail closed while a RunView exists; SQLite tests pass.** No commit or staging was performed. Integration worktree HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`.

## Scope and result

Edited only the four existing RunView migrations and `internal/application/repository/craft_run_view_test.go`. Both dialect up migrations now use `ON DELETE RESTRICT` on `(tenant_id, run_id)`. Down migrations still drop only `craft_run_views`.

The new tests cover both unresolved `allocating` and concrete `bound` rows. Direct Run deletion and Session deletion (which cascades to its Run) each fail, and reload returns the exact same generation/runtime identity. A Run without a view still deletes successfully. Existing allocation, reload, binding, scope, and migration up/down/reapply tests remain covered.

## RED → GREEN and verification

Before the SQL change, ran:

```text
gofmt -w internal/application/repository/craft_run_view_test.go
go test ./internal/application/repository -run 'TestCraftRunViewRetention' -count=1
```

RED: all four deletion tests failed because no deletion error occurred (the CASCADE removed the view); the no-view Run deletion test passed.

After changing both up migrations, ran:

```text
go test ./internal/application/repository -run 'TestCraftRunViewRetention|TestCraftRunViewMigrationAbsentThenUpDownUp|TestCraftRunViewAllocationRetryScopeAndReload|TestCraftRunViewRuntimeBindingIsCompareAndSwapAndUnresolvedStaysPending' -count=1
go test ./internal/application/repository -count=1
git diff --check
```

GREEN: focused retention/migration/allocation/bind tests passed; full repository package passed (`150.166s`); `git diff --check` passed. The migration test verified absent at 112, up to 113, down, then reapply.

PostgreSQL execution was unavailable: `TRPC_TEST_POSTGRES_DSN` was unset and `psql` was unavailable. PG RESTRICT behavior is therefore **not verified in this task**; its DDL was manually checked. The prior review's PG17.9 test applied the earlier CASCADE DDL and does not establish the changed RESTRICT behavior.

## Checkpoint

- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged).
- No commit or staging by this task. The four migration files were already intent-to-add at task start; that index state was preserved. The focused Go test is untracked.
- Full-content SHA-256:

```text
fe3b3d4a3f1dcd4277ad581538d56dec8e893b4940ebc6fc3db3e3d618d85cea  migrations/versioned/000192_craft_run_views.up.sql
e0a56197239a5500d38cf2eb11a4abb7dff02b1206b4261219881f8013b62e66  migrations/versioned/000192_craft_run_views.down.sql
52f5dedfb671ba213884333af9e2d7724956da2b51aa115bb431479b1b266990  migrations/sqlite/000113_craft_run_views.up.sql
e0a56197239a5500d38cf2eb11a4abb7dff02b1206b4261219881f8013b62e66  migrations/sqlite/000113_craft_run_views.down.sql
6923cc8e37bb031f4832a4c4ce74a30cdb47149a723675ef720379db7b172cba  internal/application/repository/craft_run_view_test.go
```

The four SQL files are `git add -N -f` intent-to-add entries, not staged content. They are retained below in full so the independent reviewer can inspect exact DDL. Binding retention does not stop or clean up any external process; verified lifecycle cleanup is a separate required operation.

## Exact SQL checkpoint

### `migrations/versioned/000192_craft_run_views.up.sql`

```sql
-- Server-owned, Run-scoped execution view identity. This row records binding
-- state only; it does not provide a filesystem or process isolation boundary.
CREATE TABLE craft_run_views (
    tenant_id BIGINT NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    generation VARCHAR(64) NOT NULL,
    runtime_id VARCHAR(255) NOT NULL DEFAULT '',
    container_id VARCHAR(255) NOT NULL DEFAULT '',
    opencode_session_id VARCHAR(255) NOT NULL DEFAULT '',
    state VARCHAR(16) NOT NULL DEFAULT 'allocating',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id),
    CONSTRAINT uq_craft_run_views_generation UNIQUE (generation),
    CONSTRAINT ck_craft_run_views_state CHECK (state IN ('allocating', 'bound')),
    CONSTRAINT ck_craft_run_views_binding CHECK (
        (state = 'allocating' AND runtime_id = '' AND container_id = '' AND opencode_session_id = '') OR
        (state = 'bound' AND runtime_id <> '' AND container_id <> '' AND opencode_session_id <> '')
    ),
    CONSTRAINT fk_craft_run_views_run FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE RESTRICT
);

CREATE INDEX idx_craft_run_views_recovery ON craft_run_views (state, updated_at);
```

### `migrations/versioned/000192_craft_run_views.down.sql`

```sql
DROP TABLE IF EXISTS craft_run_views;
```

### `migrations/sqlite/000113_craft_run_views.up.sql`

```sql
-- Server-owned, Run-scoped execution view identity. This row records binding
-- state only; it does not provide a filesystem or process isolation boundary.
CREATE TABLE craft_run_views (
    tenant_id INTEGER NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    generation VARCHAR(64) NOT NULL,
    runtime_id VARCHAR(255) NOT NULL DEFAULT '',
    container_id VARCHAR(255) NOT NULL DEFAULT '',
    opencode_session_id VARCHAR(255) NOT NULL DEFAULT '',
    state VARCHAR(16) NOT NULL DEFAULT 'allocating',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id),
    UNIQUE (generation),
    CHECK (state IN ('allocating', 'bound')),
    CHECK (
        (state = 'allocating' AND runtime_id = '' AND container_id = '' AND opencode_session_id = '') OR
        (state = 'bound' AND runtime_id <> '' AND container_id <> '' AND opencode_session_id <> '')
    ),
    FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE RESTRICT
);

CREATE INDEX idx_craft_run_views_recovery ON craft_run_views (state, updated_at);
```

### `migrations/versioned/000192_craft_run_views.down.sql` and `migrations/sqlite/000113_craft_run_views.down.sql`

```sql
DROP TABLE IF EXISTS craft_run_views;
```
