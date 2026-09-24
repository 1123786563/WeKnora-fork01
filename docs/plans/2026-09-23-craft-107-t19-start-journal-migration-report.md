# T19 charge start journal migration checkpoint

Status: DONE_WITH_CONCERNS

Checkpoint ID: `t19-charge-start-journal-000191-000112`

## Scope

Created only the four assigned SQL migration files. Schema follows the proposal in the T19 fix2 report in the T19 Worktree and the integration migration plan. The table contains tenant/run/activity identity, grant/call/reservation identities, state, Run revision and timestamps; it uses the requested composite primary key, tenant-scoped unique reservation key, four-state check, composite Run FK and unresolved scan index. The FK has no cascade action, preserving journal rows against Run deletion until explicit cleanup. This schema alone does not close T19 F3 or authorize a start.

## SQLite verification

Environment: Python 3.9.6 `sqlite3`, SQLite 3.54.0, disposable in-memory database with `PRAGMA foreign_keys=ON`. Applied all 96 existing SQLite `.up.sql` migrations below 000112, through the preceding tip 000111. Before running RED, created a valid session/Run and an existing `commercial_reservations` row.

Exact output:

```text
RED: journal table absent at SQLite tip 000111; existing agent_run and reservation readable
GREEN: journal constraints, all four allowed states, unresolved indexed scan, existing Run/reservation preserved
UNRESOLVED QUERY PLAN: 4 0 71 SEARCH craft_charge_start_journal USING INDEX idx_craft_charge_start_journal_state_updated_at (state=?) 29 0 0 USE TEMP B-TREE FOR ORDER BY
ROLLBACK: only journal/index removed; existing Run/reservation preserved
REAPPLY: migration applies successfully
SQLite version: 3.54.0 preceding migrations applied: 96
```

GREEN inserted all four allowed states and confirmed the scan for `intent` and `unknown` uses `idx_craft_charge_start_journal_state_updated_at`. Duplicate `(tenant_id, run_id, activity_key)`, duplicate `(tenant_id, reservation_key)`, invalid state, and a missing composite Run FK target each raised `sqlite3.IntegrityError`. Existing `agent_runs` and commercial reservation rows remained unchanged through apply and rollback.

## PostgreSQL and whitespace checks

- `psql` was unavailable on the host, so ran the actual PG up/down SQL in a disposable schema on local Docker `WeKnora-postgres-dev` (PostgreSQL 17), inside one transaction that was rolled back. The test created an `agent_runs` composite key using the repository's `INTEGER` / `VARCHAR(64)` key types, applied the migration, inserted/read an `intent`, applied down, and asserted the journal table was absent. This verified FK type compatibility and both migration directions.
- Exact output: `PostgreSQL 17 transactional up/insert/read/down passed; transaction rolled back` (exit 0).
- Ran per-file `git diff --no-index --check /dev/null <migration-file>` for each of the four ignored SQL files; expected new-file diff status 1, with no whitespace diagnostics.
- Ran `git diff --check`; exit 0.

## Checkpoint state

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD at capture: `a5e9195acd6500c085c85d60c852148e7bbbbf34`.
- `migrations/` is ignored by `.gitignore:96`; ordinary Git status omits the four SQL files. Controller must explicitly capture/add them for review.
- No commit was made.
- T19 F3 remains open; no recovery/coordinator implementation was performed by this schema task.

## SHA-256 manifest

```text
0da07aba490088dd1482a4b932680192c71848d126d98f4191d360cde045e08d  migrations/versioned/000191_craft_charge_start_journal.up.sql
c4c99c1043d548446a3bb7b1f9c78de7c600a1f449bfd0d456558775c7257bc6  migrations/versioned/000191_craft_charge_start_journal.down.sql
f3be6755579fe4545c4555894ef19ea754cf44eb178db6e691c68faa5f5f45c3  migrations/sqlite/000112_craft_charge_start_journal.up.sql
c4c99c1043d548446a3bb7b1f9c78de7c600a1f449bfd0d456558775c7257bc6  migrations/sqlite/000112_craft_charge_start_journal.down.sql
```

## Full SQL content checkpoint

### `migrations/versioned/000191_craft_charge_start_journal.up.sql`

```sql
-- Durable tenant-scoped journal for chargeable external start attempts.
CREATE TABLE craft_charge_start_journal (
    tenant_id       BIGINT       NOT NULL,
    run_id          VARCHAR(128) NOT NULL,
    activity_key    VARCHAR(256) NOT NULL,
    grant_id        VARCHAR(128) NOT NULL,
    call_id         VARCHAR(256) NOT NULL,
    reservation_key VARCHAR(320) NOT NULL,
    state           VARCHAR(32)  NOT NULL,
    run_revision    BIGINT       NOT NULL,
    created_at      TIMESTAMP    NOT NULL,
    updated_at      TIMESTAMP    NOT NULL,
    PRIMARY KEY (tenant_id, run_id, activity_key),
    UNIQUE (tenant_id, reservation_key),
    CHECK (state IN ('intent', 'started', 'unknown', 'definitely_unstarted')),
    FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id)
);

CREATE INDEX idx_craft_charge_start_journal_state_updated_at
    ON craft_charge_start_journal (state, updated_at);
```

### `migrations/versioned/000191_craft_charge_start_journal.down.sql`

```sql
DROP INDEX IF EXISTS idx_craft_charge_start_journal_state_updated_at;
DROP TABLE IF EXISTS craft_charge_start_journal;
```

### `migrations/sqlite/000112_craft_charge_start_journal.up.sql`

```sql
-- Durable tenant-scoped journal for chargeable external start attempts.
CREATE TABLE craft_charge_start_journal (
    tenant_id       BIGINT       NOT NULL,
    run_id          VARCHAR(128) NOT NULL,
    activity_key    VARCHAR(256) NOT NULL,
    grant_id        VARCHAR(128) NOT NULL,
    call_id         VARCHAR(256) NOT NULL,
    reservation_key VARCHAR(320) NOT NULL,
    state           VARCHAR(32)  NOT NULL,
    run_revision    BIGINT       NOT NULL,
    created_at      DATETIME    NOT NULL,
    updated_at      DATETIME    NOT NULL,
    PRIMARY KEY (tenant_id, run_id, activity_key),
    UNIQUE (tenant_id, reservation_key),
    CHECK (state IN ('intent', 'started', 'unknown', 'definitely_unstarted')),
    FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id)
);

CREATE INDEX idx_craft_charge_start_journal_state_updated_at
    ON craft_charge_start_journal (state, updated_at);
```

### `migrations/sqlite/000112_craft_charge_start_journal.down.sql`

```sql
DROP INDEX IF EXISTS idx_craft_charge_start_journal_state_updated_at;
DROP TABLE IF EXISTS craft_charge_start_journal;
```
