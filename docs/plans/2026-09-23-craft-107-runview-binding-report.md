# Craft RunView Binding Task Report

Status: **RunView identity, persistence and compare-and-swap API implemented.** This is a binding prerequisite only, not OS isolation or T01/T05 acceptance. No commit was created. Integration worktree HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`.

## Scope

This task added only the planned new files:

- `migrations/versioned/000192_craft_run_views.{up,down}.sql`
- `migrations/sqlite/000113_craft_run_views.{up,down}.sql`
- `internal/application/repository/craft_run_view.go` and `craft_run_view_test.go`
- `internal/modules/craft/run_view.go` and `run_view_test.go`

The migration files are ignored by `.gitignore:96` (`migrations/`). They were deliberately not force-added or staged. Their exact full contents are preserved below with SHA-256 hashes; they need intent-to-add before OCR/review or delivery can include them.

The repository validates `(tenant, owner, session, run)` against the authoritative `agent_runs` row. Allocation is one `INSERT ... SELECT` guarded by the exact admitted scope and unique `(tenant_id, run_id)`; it uses 32 cryptographically random bytes for an opaque `rv_` generation. Same-scope concurrent/retry allocation returns that same row. Cross-tenant/session reads are not found; a different owner in the same session is forbidden. Runtime/container/OpenCode session identity binds together with a compare-and-swap; exact replay is idempotent, different values/generation conflict. An unresolved allocation remains `allocating`, with no partial runtime identity and no replacement generation.

No absolute filesystem path is stored. Runtime IDs are opaque identifiers only. The binding row and `x-opencode-directory` header do not provide filesystem/process isolation; a server-side authenticated allocator, idempotent recovery for uncertain external creation, scoped event delivery, and per-Run runtime isolation remain separate work. If container/session creation may have succeeded but its response was lost before `BindRuntime`, the caller must resolve that outcome using the stable generation before attempting any new creation; this store cannot identify an external resource whose ID was never returned to it.

## RED → GREEN and verification

RED was run before the contract/store implementation:

```text
go test ./internal/modules/craft ./internal/application/repository -run 'TestValidateRunView|TestCraftRunView' -count=1
```

It failed to compile at the expected missing `RunViewKey`, `ValidateRunView`, `RunViewStore`, and `NewCraftRunViewStore` references.

Focused GREEN run after implementation:

```text
go test -v ./internal/modules/craft ./internal/application/repository -run 'TestValidateRunView|TestCraftRunView' -count=1
```

Passed: both typed contract tests, SQLite migration absent/up/down/reapply, allocation/retry/scope/reopen tests, two-connection allocation race, and binding-CAS/unresolved-state tests. The three PostgreSQL subtests were skipped because `TRPC_TEST_POSTGRES_DSN` is unset.

Final repository package run:

```text
go test ./internal/application/repository -count=1
```

Passed (`202.426s`).

A broader `go test ./internal/modules/craft -count=1` run failed in the unrelated concurrent T05 file `knowledge_test.go`, test `TestExcerptOfBoundsAtRuneBoundary` (`overshoot marker missing: "知知"`). The RunView tests in that package passed in the focused run. No T05 source was changed.

`git diff --check` passed. PostgreSQL DDL was manually checked for parity with `agent_runs` keys and migration order; neither `psql` nor `TRPC_TEST_POSTGRES_DSN` was available, so PostgreSQL execution remains unverified.

## Checkpoint

- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged).
- No commit; no files staged by this task.
- The shared integration worktree had unrelated edits at task start and continued to receive unrelated parallel changes; all were preserved.
- Full-content SHA-256:

```text
022f45f10e7a4e3c137945758cfa84be80417a66ce0e7d672962ec14da5259c9  migrations/versioned/000192_craft_run_views.up.sql
e0a56197239a5500d38cf2eb11a4abb7dff02b1206b4261219881f8013b62e66  migrations/versioned/000192_craft_run_views.down.sql
2d34e12f3edf2a0f68b810d466676f8f6fde6fdffbae27fa0d2f56ce98133ec2  migrations/sqlite/000113_craft_run_views.up.sql
e0a56197239a5500d38cf2eb11a4abb7dff02b1206b4261219881f8013b62e66  migrations/sqlite/000113_craft_run_views.down.sql
8675fed32bbd5d63bc6a6a0a51276434d8bb81c9ceab07346a6b6d2315388c73  internal/application/repository/craft_run_view.go
35bad5176906111b8ff232d9d7e2f07a162c70a6dd6c10c39a84c7d7a19b8563  internal/application/repository/craft_run_view_test.go
1b4fd31390b287423dd3ec61f28aac4837c05109cf6fb8bcdc749d686b27d5f1  internal/modules/craft/run_view.go
e69edbc368b1a3100c338e77229a9d0b6799e60d94bb8336135251fc1fcb2cfb  internal/modules/craft/run_view_test.go
```

## Ignored migration source content

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
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE
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
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE
);

CREATE INDEX idx_craft_run_views_recovery ON craft_run_views (state, updated_at);
```

### `migrations/sqlite/000113_craft_run_views.down.sql`

```sql
DROP TABLE IF EXISTS craft_run_views;
```
