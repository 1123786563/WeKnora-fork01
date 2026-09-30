# T01 input admission fence migration checkpoint

Status: DONE_WITH_CONCERNS

Checkpoint ID: `t01-admission-migration-000190-000111`

## Scope and source

Created only the four assigned SQL migrations. The referenced T01 fix2 report was not present in the integration Worktree; implementation follows the complete field definitions in `2026-09-23-craft-107-t01-admission-migration-plan.md`:

- nullable `admission_run_id VARCHAR(64)`
- nullable `admission_token VARCHAR(64)`
- `admission_state VARCHAR(16) NOT NULL DEFAULT ''`
- nullable `lease_expires_at TIMESTAMP` / `DATETIME`

No check constraint or index was added because the plan says these are optional only if supported consistently. Legacy `input_decision` rows remain fail-closed with NULL identifiers/lease and empty state. This migration does not implement or claim to solve T01 admission recovery.

## SQLite RED / GREEN / rollback / reapply

Environment: Python 3.9.6 `sqlite3`, SQLite 3.54.0, disposable in-memory database. The fixture used the existing `craft_session_requests` columns/primary key from migration 000049 and inserted one `purpose='input_decision'` legacy row before the migration.

Exact successful output:

```text
RED: admission columns absent; legacy input_decision row present
GREEN: all fields have required nullability/default; legacy remains null/empty; admission claim insert/lookup passes
ROLLBACK: admission columns removed; legacy input_decision row preserved
REAPPLY: migration applies again successfully
SQLite version: 3.54.0
```

The GREEN check inspected `PRAGMA table_info` for nullability/defaults, confirmed the legacy row had `(NULL, NULL, '', NULL)`, and inserted/looked up an `input_admission` claim with the four fields populated. The down migration removed only the four new columns while preserving the legacy row.

The first test attempt used an unfiltered SELECT after inserting the claim and therefore hit an order-dependent assertion at the rollback step. I corrected it to select the legacy row by its request key and reran the complete test successfully; this did not require a migration change.

## PostgreSQL and whitespace checks

- Checked `command -v psql` and `command -v pg_format`; neither tool is available. No PostgreSQL parser or live database execution was possible, so PostgreSQL syntax remains unverified by execution.
- Ran per-file `git diff --no-index --check /dev/null <migration-file>` for all four ignored migration files; each returned the expected new-file diff status 1 with no whitespace diagnostics.
- Ran `git diff --check`; exit 0.

## Checkpoint state

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`
- Exact `git status --short --untracked-files=all` for the four SQL paths: no entries, because `.gitignore:96` ignores `migrations/`.
- Report status: untracked `docs/plans/2026-09-23-craft-107-t01-admission-migration-report.md`.
- No commit was made.

## SHA-256 manifest

```text
7ded4b7d201936ab211dc934416efde9295967354df5ca4d0cc3c6601af3634f  migrations/versioned/000190_craft_input_admission_fence.up.sql
dff4ada13df499b2f6fcafbdf57f91b31f316c22b428c0d1e72f9061af556ec7  migrations/versioned/000190_craft_input_admission_fence.down.sql
d59894a3b9b318a9f182f0b23eb69d689547599c21ead5249d4cdcb1f961fc18  migrations/sqlite/000111_craft_input_admission_fence.up.sql
dff4ada13df499b2f6fcafbdf57f91b31f316c22b428c0d1e72f9061af556ec7  migrations/sqlite/000111_craft_input_admission_fence.down.sql
```

## Full SQL content checkpoint

### `migrations/versioned/000190_craft_input_admission_fence.up.sql`

```sql
-- T01 durable input-admission claim fields. Legacy rows remain fail-closed.
ALTER TABLE craft_session_requests
    ADD COLUMN admission_run_id VARCHAR(64);
ALTER TABLE craft_session_requests
    ADD COLUMN admission_token VARCHAR(64);
ALTER TABLE craft_session_requests
    ADD COLUMN admission_state VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE craft_session_requests
    ADD COLUMN lease_expires_at TIMESTAMP;
```

### `migrations/versioned/000190_craft_input_admission_fence.down.sql`

```sql
ALTER TABLE craft_session_requests DROP COLUMN lease_expires_at;
ALTER TABLE craft_session_requests DROP COLUMN admission_state;
ALTER TABLE craft_session_requests DROP COLUMN admission_token;
ALTER TABLE craft_session_requests DROP COLUMN admission_run_id;
```

### `migrations/sqlite/000111_craft_input_admission_fence.up.sql`

```sql
-- T01 durable input-admission claim fields. Legacy rows remain fail-closed.
ALTER TABLE craft_session_requests ADD COLUMN admission_run_id VARCHAR(64);
ALTER TABLE craft_session_requests ADD COLUMN admission_token VARCHAR(64);
ALTER TABLE craft_session_requests ADD COLUMN admission_state VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE craft_session_requests ADD COLUMN lease_expires_at DATETIME;
```

### `migrations/sqlite/000111_craft_input_admission_fence.down.sql`

```sql
ALTER TABLE craft_session_requests DROP COLUMN lease_expires_at;
ALTER TABLE craft_session_requests DROP COLUMN admission_state;
ALTER TABLE craft_session_requests DROP COLUMN admission_token;
ALTER TABLE craft_session_requests DROP COLUMN admission_run_id;
```
