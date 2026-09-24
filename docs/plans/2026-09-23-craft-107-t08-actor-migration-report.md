# T08 Actor Column Migration Report

**Status:** DONE_WITH_CONCERNS — schema prerequisite implemented and verified; this migration alone does not complete collaborator security or authorize collaborator execution.

## Scope and checkpoint

Added only the four requested SQL migration files and this report. Checked before writing: PostgreSQL target `000194` and SQLite target `000115` did not exist; the latest preceding migration numbers were PostgreSQL `000193` and SQLite `000114`. These four migration files are ignored by `.gitignore` rule `.gitignore:96:migrations/`; they are intentionally unstaged and untracked from Git’s view. No commit was created.

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`
- SQLite up SHA-256: `9ebc9b93902ba0e893d302b2371fffae4b074cec7e5c61102005bf5a9bbd0b28`
- SQLite down SHA-256: `9d9a2350e3d176b4237c62c5a188177c12077cf40596485ee29d4589fc69a589`
- PostgreSQL up SHA-256: `9ebc9b93902ba0e893d302b2371fffae4b074cec7e5c61102005bf5a9bbd0b28`
- PostgreSQL down SHA-256: `cb429432a13c89f4de482d340f13a45ca09d6c5b1abd4db117805091d204e115`

## RED → GREEN verification

- RED: disposable SQLite schema confirmed `actor_user_id` absent before migration and contained a preexisting row with `owner_id='legacy-owner'`.
- GREEN SQLite: applied up; confirmed `actor_user_id` is nullable `VARCHAR(512)` and the existing row remained intact with NULL actor; inserted and read back an actor value longer than 36 characters; applied down and confirmed the actor column alone was removed while the old row remained; reapplied up and confirmed the legacy actor remained NULL.
- GREEN PostgreSQL: executed both migration directions and reapply inside a transaction against a temporary `agent_runs` table on PostgreSQL `17.9`; asserted nullable `VARCHAR(512)`, unchanged legacy row/NULL actor, inserted and read back an actor value longer than 36 characters, successful down and reapply, then rolled back the transaction. No persistent database state was changed.
- All four migration files passed the whitespace scan. No `git diff --check` diagnostics were found for these ignored files; Git’s no-index diff returns 1 by design because each file differs from `/dev/null`.

## Full SQL contents

### `migrations/versioned/000194_craft_run_actor.up.sql`

```sql
-- Store the authenticated initiating user separately from the Task storage owner.
-- Historical actor identity remains unknown; do not backfill from owner_id.
ALTER TABLE agent_runs
    ADD COLUMN actor_user_id VARCHAR(512) NULL;
```

### `migrations/versioned/000194_craft_run_actor.down.sql`

```sql
ALTER TABLE agent_runs
    DROP COLUMN IF EXISTS actor_user_id;
```

### `migrations/sqlite/000115_craft_run_actor.up.sql`

```sql
-- Store the authenticated initiating user separately from the Task storage owner.
-- Historical actor identity remains unknown; do not backfill from owner_id.
ALTER TABLE agent_runs
    ADD COLUMN actor_user_id VARCHAR(512) NULL;
```

### `migrations/sqlite/000115_craft_run_actor.down.sql`

```sql
ALTER TABLE agent_runs
    DROP COLUMN actor_user_id;
```

## Scope limits

No owner-to-actor backfill, index, repository/service change, RunView change, migration registration/ledger change, or collaborator security completion was included. Existing legacy rows remain actor-unknown. Repository code must later require and atomically persist the authenticated actor for newly admitted Craft Runs before collaborator runs are authorized.
