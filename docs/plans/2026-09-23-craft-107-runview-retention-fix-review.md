# RunView retention fix independent review

Date: 2026-09-23. Read-only scoped review at integration HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Sources: approved Craft Spec/#120/#124, ADR-0004, `CONTEXT.md`, RunView binding design/plan and prior independent binding review, retention fix plan and implementation report. No OCR or source/test/SQL edits were made.

## Exact checkpoint

The five live files match the report's full-content SHA-256. The four SQL paths remain Git intent-to-add (` A`, not staged content); the test is untracked (`??`). Their inclusion in the eventual delivery must be checked explicitly.

| File | SHA-256 |
| --- | --- |
| `migrations/versioned/000192_craft_run_views.up.sql` | `fe3b3d4a3f1dcd4277ad581538d56dec8e893b4940ebc6fc3db3e3d618d85cea` |
| `migrations/versioned/000192_craft_run_views.down.sql` | `e0a56197239a5500d38cf2eb11a4abb7dff02b1206b4261219881f8013b62e66` |
| `migrations/sqlite/000113_craft_run_views.up.sql` | `52f5dedfb671ba213884333af9e2d7724956da2b51aa115bb431479b1b266990` |
| `migrations/sqlite/000113_craft_run_views.down.sql` | `e0a56197239a5500d38cf2eb11a4abb7dff02b1206b4261219881f8013b62e66` |
| `internal/application/repository/craft_run_view_test.go` | `6923cc8e37bb031f4832a4c4ce74a30cdb47149a723675ef720379db7b172cba` |

## Verdict

- **Scoped Spec compliance: PASS**, conditional on these migration numbers being unapplied in every target database. Both up migrations now use a composite `(tenant_id, run_id)` Run FK with `ON DELETE RESTRICT` (`PG:22-23`, `SQLite:22-23`). The row retains its generation and external identity if a direct Run deletion or a parent Session cascade is attempted. This closes the prior binding review's automatic deletion finding at the database boundary.
- **Scoped code quality: PASS**, with the migration deployment condition below. The tests cover allocating/bound views, direct Run and Session deletion, exact postfailure reload, and deletion of a Run without a view (`craft_run_view_test.go:210-254`). Both down migrations drop only the owned table. No store API or unrelated migration was changed.
- **T01/T05 end-to-end acceptance: NOT VERIFIED.** A retained database row is not a per-Run filesystem/process isolation boundary and does not stop an external runtime. There is no verified terminal/retention cleanup path in this scope. It is expected that deleting even a terminal Run with a view now fails until an explicit reconciled cleanup removes that view.

## Finding / deployment condition

### Medium, conditional — changing an already applied migration leaves CASCADE in place

**Evidence and affected files:** The only DDL correction is inside existing version `000192`/`000113` up files (`:22-23`). A migration runner that has recorded either version as applied does not replay the edited file. Such a database still has the old CASCADE FK, and the fresh database tests (`craft_run_view_test.go:210-254`) would not detect this drift. The plan explicitly says these migrations have not shipped; that condition must be established for every deployment target, including shared development and staging databases.

**Impact:** On a previously migrated database, Run/Session deletion can still erase a view and its generation/runtime identity despite the source showing `RESTRICT`.

**Smallest correction:** Before delivery, verify the migration version and actual FK action in every target database. If any persistent target has applied the CASCADE version, ship a new corrective migration (PostgreSQL constraint replacement; SQLite table rebuild with FK checks), rather than relying on edited historical up files. Also include all four intent-to-add SQL files in the delivery artifact. This is a release condition, not evidence that the scoped unshipped-file fix is wrong.

## Independent verification and limits

`go test ./internal/application/repository -run 'TestCraftRunViewRetention|TestCraftRunViewMigrationAbsentThenUpDownUp' -count=1` passed (`ok`, 3.937 s). The implementation report records the RED failure of all four view deletion cases under CASCADE, the GREEN focused/full repository package pass, and `git diff --check`; I did not rerun its full 150-second suite.

I applied the exact PostgreSQL up and down SQL in a rolled-back disposable schema on the local PostgreSQL 17 container. With a Session → Run `ON DELETE CASCADE` FK and an allocating view, both direct Run deletion and Session deletion raised foreign-key violations; all three rows remained (`1|1|1`). After the down migration, Session deletion cascaded to its Run; psql exited 0 with `ON_ERROR_STOP=1`. This establishes PostgreSQL DDL and basic deletion semantics, not migration against an existing deployed schema, runtime cleanup, OS isolation, or PostgreSQL repository concurrency behavior. The SQLite focused test establishes the same requested deletion states with the project's test database and FK enforcement.
