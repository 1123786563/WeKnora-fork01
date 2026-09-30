# T08 membership migration checkpoint report

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- Assignment role: `mechanical_worker` (per parent task). Runtime model and effort were not exposed in this execution context.
- Change: added `membership_id BIGINT NOT NULL` to PostgreSQL `craft_task_grants` and `membership_id INTEGER NOT NULL` to SQLite `craft_task_grants`, immediately after `tenant_id` in each up migration.
- Scope: down migrations, existing columns, primary key, index, other tables, and production code were not changed. No commit created.

## Verification

- Disposable SQLite baseline checkpoint migration applied successfully; the pre-change grant table has no `membership_id`.
- Disposable SQLite updated up migration applied successfully. Verified `membership_id` is `INTEGER NOT NULL`, a row with membership ID 42 can be inserted and read back, and an insert omitting it is rejected.
- Verified the existing recognition columns remain nullable, the original composite primary key and tenant/user index remain, and rollback removes the grant/knowledge tables and recognition columns.
- `git diff --no-index --check /dev/null <file>` run for each of the four ignored SQL files; no whitespace errors. Exit status 1 is the expected indication that each file differs from `/dev/null`.
- PostgreSQL syntax reviewed from the migration text; no live PostgreSQL server check was run.

## Full file SHA-256

| File | SHA-256 |
| --- | --- |
| `migrations/versioned/000189_craft_web_artifact_frontier.up.sql` | `9a5b429f34e9e746573defc63ff5fd1302850c83e08fbe0ae416ab17532ba04c` |
| `migrations/versioned/000189_craft_web_artifact_frontier.down.sql` | `968a65b22b5f5e7a4d1ef370a7d198a6b7f1774aadac8d75bb861228b3010f0e` |
| `migrations/sqlite/000110_craft_web_artifact_frontier.up.sql` | `b58a351d5b69eaad4f192b9ddaf533fdd5ac79825e71c30fff6c80263359e6f8` |
| `migrations/sqlite/000110_craft_web_artifact_frontier.down.sql` | `968a65b22b5f5e7a4d1ef370a7d198a6b7f1774aadac8d75bb861228b3010f0e` |
