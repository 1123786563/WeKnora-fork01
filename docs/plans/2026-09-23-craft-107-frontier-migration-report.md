# Craft #107 first-frontier migration report

Status: DONE_WITH_CONCERNS

## Changed files

- `migrations/versioned/000189_craft_web_artifact_frontier.up.sql`
- `migrations/versioned/000189_craft_web_artifact_frontier.down.sql`
- `migrations/sqlite/000110_craft_web_artifact_frontier.up.sql`
- `migrations/sqlite/000110_craft_web_artifact_frontier.down.sql`

No application code, prior migrations, migration registry, or other agents' files were changed. A focused disposable SQLite verification was used, so no verification source file was added.

## Schema delivered

- Added nullable `recognition_accepted BOOLEAN`, `recognition_understood BOOLEAN`, and `recognition_reason TEXT` to `craft_workspace_inputs`; no defaults are assigned.
- Added `craft_knowledge_records` with the requested tenant/session/run primary key and fields, without a source foreign key or extra index.
- Added `craft_task_grants` with the requested tenant/session/user primary key and `idx_craft_task_grants_tenant_user` on `(tenant_id, user_id)`.
- Down migrations drop the grant index, both tables, and all three recognition columns.

## Verification evidence

Executed the following disposable SQLite procedure with Python 3.9.6 / sqlite3: created a minimal `sessions` table, applied `migrations/sqlite/000045_craft.up.sql` and `000049_craft_sessions.up.sql`, inserted a workspace and a legacy input row, then checked the new migration before and after application.

- RED: confirmed all three recognition columns and both durable tables were absent before migration.
- GREEN: applied `migrations/sqlite/000110_craft_web_artifact_frontier.up.sql`; confirmed all three recognition columns, both durable tables, and `idx_craft_task_grants_tenant_user` exist.
- Legacy recognition: queried the pre-existing input after migration; values were `(NULL, NULL, NULL)`.
- Rollback: applied `migrations/sqlite/000110_craft_web_artifact_frontier.down.sql`; confirmed recognition columns, both tables, and the index were removed.
- `git diff --check`: exit 0.
- The repository ignores `migrations/` (`.gitignore:96`), so the new SQL files are not shown by ordinary `git status`. Checked each ignored SQL file with `git diff --no-index --check /dev/null <file>` (accepted diff exit 1 for a new file; no whitespace diagnostics), then ran `git diff --check` for the visible report; final command exit 0. The controller must explicitly include the ignored migration files in its snapshot/copy operation.

Exact verification output:

```text
RED: expected missing recognition columns and durable tables confirmed
GREEN: recognition columns, both tables and grant index exist; legacy recognition remains NULL
ROLLBACK: columns, tables and index removed
git diff --check exit=0
```

## Limitations

PostgreSQL DDL was not executed against a PostgreSQL database in this focused run. The migration follows the existing PostgreSQL migration dialect and the required definitions, but live PostgreSQL acceptance remains for the controller's available migration test path.

## SHA-256

```text
8719d24a340e8eca8e077f37371e6855e647b0d29382f0571282d7919d0e5323  migrations/versioned/000189_craft_web_artifact_frontier.up.sql
968a65b22b5f5e7a4d1ef370a7d198a6b7f1774aadac8d75bb861228b3010f0e  migrations/versioned/000189_craft_web_artifact_frontier.down.sql
bbdaf479acb52e4b72a676cdd03f15686124338a8e79a91960139934ce6f9706  migrations/sqlite/000110_craft_web_artifact_frontier.up.sql
968a65b22b5f5e7a4d1ef370a7d198a6b7f1774aadac8d75bb861228b3010f0e  migrations/sqlite/000110_craft_web_artifact_frontier.down.sql
```
