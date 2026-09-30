# RunView Binding Retention Fix Plan

> **For Codex:** Execute through SDD with RED → GREEN → REFACTOR, exact uncommitted checkpoint and independent Review. Do not commit.

**Goal:** Prevent deleting a Run from silently erasing an unresolved or bound RunView and its generation/runtime identity. This closes the Medium finding in `2026-09-23-craft-107-runview-binding-review.md`; it does not claim per-Run OS isolation.

**Sources:** Approved Spec #107/#120/#124, `2026-09-23-craft-107-per-run-isolation-design.md` in T01 Worktree, RunView binding plan/report/review, PG 000192 and SQLite 000113 migrations. Integration Worktree HEAD `a5e9195...` plus uncommitted files; all four SQL currently `git add -N -f` only.

**Global Constraints:** backend_implementer owns only the four 000192/000113 SQL files, `internal/application/repository/craft_run_view_test.go`, and focused repository test support if strictly necessary. No changes to the RunView store API, T01 service, T05 Publisher, container, OpenCode client, other migrations, commits or shared agent files. Other agents work concurrently in the integration Worktree. Preserve their changes. If a cleanup API becomes necessary, report that to controller as a separate plan; do not broaden ownership silently.

**Review Focus:** A Run or Session deletion must fail while any RunView row exists, retaining generation and external identity for reconciliation. Restrictive FK behavior on PostgreSQL and SQLite with `PRAGMA foreign_keys=ON`; migration up/down/reapply, allocation/binding and cross-scope tests still pass. Never assume DB row deletion terminates the external process. Existing migration has not shipped; replace its `ON DELETE CASCADE` with `ON DELETE RESTRICT`/equivalent within these same migration numbers. Recognize that even a terminal Run's view needs explicit verified cleanup later; this fix is fail-closed retention only.

## Task 1 — restrict RunView deletion

**Depends on:** reviewed RunView binding checkpoint; no other file prerequisites. **Produces:** database-enforced retention of view identity.

1. RED: add SQLite test allocating a view, then attempt direct Run deletion and parent Session cascade; both must fail and leave the exact view generation/runtime identity intact. Add test for bound and unresolved allocating states, and no-view Run deletion compatibility. Run it against current CASCADE and record failing output.
2. GREEN: alter both PG/SQLite 000192/000113 up migrations to a restrictive FK. Keep down migrations dropping only their own table/index. Test SQLite migration absent/up/down/reapply and, when available, PostgreSQL 17 same deletion behavior in a disposable schema/transaction. Report any skipped PG test explicitly.
3. REFACTOR: focused and full repository package, `git diff --check`, exact full-content/hash checkpoint including all four ignored SQL files, task report. Independent reviewer checks exact SQL and deletion tests before controller considers this finding closed.
