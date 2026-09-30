# Craft #107 first-frontier migration independent review

**Scope:** Exactly the four ignored SQL files `migrations/versioned/000189_craft_web_artifact_frontier.{up,down}.sql` and `migrations/sqlite/000110_craft_web_artifact_frontier.{up,down}.sql`, inspected from their full on-disk contents. Read-only review of production and test sources; this report is the only edit.

**Authority checked:** Approved `docs/specs/2026-09-23-craft-web-artifact-spec.md`; `docs/adr/0004-task-is-session.md`; `CONTEXT.md`; `docs/plans/2026-09-23-craft-107-frontier-migration-brief.md`; T01, T05 and T08 sections of `docs/plans/2026-09-23-craft-107-implementation.md`; T00 legacy recognition contract in `docs/plans/2026-09-23-craft-107-t00-report.md`.

## Verdict

- **Spec compliance: PASS.** Both dialects add the three nullable input recognition fields without defaults; create the specified knowledge record and task grant tables with exact primary-key scope; create only the requested grant lookup index; and reverse those changes in the down migration. Existing input rows consequently retain an unknown (`NULL`) recognition fact, consistent with T00's absent-recognition → `null` parser behavior. No source foreign key or extra index was introduced.
- **Code quality: PASS for the four SQL files.** Migration numbers are the next numbers in their respective streams (PostgreSQL 000189 after 000188; SQLite 000110 after 000109). The DDL follows existing PostgreSQL `TIMESTAMP` and SQLite `DATETIME` conventions and SQLite `DROP COLUMN` rollback patterns. The secondary `(tenant_id,user_id)` index supports the intended grant lookup. No security or integrity defect was identified in the specified schema.

## Findings

No critical, high, medium or low findings against these four files.

## Evidence and limits

- Reviewed `000189` and `000110` up/down files line by line. The table definitions are at PostgreSQL up lines 9–31 and SQLite up lines 6–28; inverse operations are at down lines 1–6 in both dialects.
- `go test ./internal/database -run '^TestSQLiteMigrationsCreateVersionedSchema$' -count=1` exited 0. This exercises the repository's full SQLite migration stream through its current head.
- Independent in-memory SQLite check applied the actual `000110` up/down SQL to a table with a preexisting input row. It confirmed `(NULL,NULL,NULL)` recognition on that row, both composite primary keys reject duplicates, the `(tenant_id,user_id)` index exists after up, and rollback removes the new columns/tables/index while preserving the old row. Check exited 0.
- PostgreSQL DDL was reviewed for syntax and parity, but no live PostgreSQL migration was run in this review. That execution remains an acceptance limit, not evidence of a defect.
- The four SQL files are ignored by Git. Their content must be explicitly included in the controller's checkpoint and eventual delivery; an ordinary `git diff` or `git status` does not prove coverage. This is an integration handling requirement, not a SQL finding.

**Reviewed SHA-256:**

```text
8719d24a340e8eca8e077f37371e6855e647b0d29382f0571282d7919d0e5323  migrations/versioned/000189_craft_web_artifact_frontier.up.sql
968a65b22b5f5e7a4d1ef370a7d198a6b7f1774aadac8d75bb861228b3010f0e  migrations/versioned/000189_craft_web_artifact_frontier.down.sql
bbdaf479acb52e4b72a676cdd03f15686124338a8e79a91960139934ce6f9706  migrations/sqlite/000110_craft_web_artifact_frontier.up.sql
968a65b22b5f5e7a4d1ef370a7d198a6b7f1774aadac8d75bb861228b3010f0e  migrations/sqlite/000110_craft_web_artifact_frontier.down.sql
```
