# T08 actor column migration independent review

**Scope:** Exact contents of PostgreSQL `000194_craft_run_actor.{up,down}.sql` and SQLite `000115_craft_run_actor.{up,down}.sql` in the integration Worktree. Reviewed against the approved Craft Spec, `CONTEXT.md`, ADR-0004/0009, `2026-09-23-craft-107-t08-actor-principal-plan.md`, actor principal design, and the migration report. This is a schema review; actor admission and worker authorization are separate implementation tasks.

## Verdict

- **Spec compliance: PASS for the migration.** Both up files add a nullable `actor_user_id VARCHAR(512)` to `agent_runs`, matching that table's `owner_id VARCHAR(512)` width in PostgreSQL `000093_agent_runs.up.sql` and SQLite `000014_agent_runs.up.sql` / `000055_workbench_runs.up.sql`. No owner field or key is changed. Nullable, unbackfilled historical actors preserve “unknown” rather than assigning the Task owner without proof, which is required by the actor plan's fail-closed legacy policy.
- **Code quality: PASS for the migration.** Each down file removes only `actor_user_id`; there is no extra index, default, backfill, or table rebuild. `000194` and `000115` follow the preceding migration numbers in their respective streams. The four live file SHA-256 values match the migration report. The ignored SQL files must be explicitly included in the controller's full-content checkpoint and final review scope.

## Findings

No critical, high, medium, or low findings in these four SQL files.

## Evidence and limits

- Independent SQLite in-memory up/down/reapply check on a table with a legacy owner row passed. The new column was nullable `VARCHAR(512)`; the old row retained its owner and `NULL` actor; a 100-character actor was stored and read; down removed only the actor column; reapply left the legacy actor `NULL`.
- The migration report records a PostgreSQL 17.9 transactional up/down/reapply check, including a preexisting row and long actor ID. This review inspected PostgreSQL SQL and schema parity but did not independently connect to PostgreSQL.
- This column alone does not make collaborator execution safe. The repository must persist the authenticated actor atomically, reject cross-actor replay, and worker recovery must use that actor for capability authority while keeping owner as the Task storage key, as specified in the actor plan.

**Reviewed SHA-256:**

```text
9ebc9b93902ba0e893d302b2371fffae4b074cec7e5c61102005bf5a9bbd0b28  migrations/versioned/000194_craft_run_actor.up.sql
cb429432a13c89f4de482d340f13a45ca09d6c5b1abd4db117805091d204e115  migrations/versioned/000194_craft_run_actor.down.sql
9ebc9b93902ba0e893d302b2371fffae4b074cec7e5c61102005bf5a9bbd0b28  migrations/sqlite/000115_craft_run_actor.up.sql
9d9a2350e3d176b4237c62c5a188177c12077cf40596485ee29d4589fc69a589  migrations/sqlite/000115_craft_run_actor.down.sql
```
