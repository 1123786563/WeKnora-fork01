# T19 Charge Start Journal Migration Plan

> **For Codex:** Execute this narrow Superpowers plan with RED migration check, GREEN SQL, rollback verification, exact checkpoint and independent Review. Do not commit.

**Goal:** Add a durable tenant-scoped charge-start journal for T19/#138 so future coordinator logic can commit intent plus G4 hold before an external model/sandbox start. Schema alone does not close F3 or authorize an external start.

**Sources:** Approved Spec #107, #138 snapshot, `docs/plans/2026-09-23-craft-107-t19-fix2-review.md` in integration; T19 fix2 report and central seam map in T19 Worktree. PG 000190/SQLite 000111 already belong to T01; this Task owns PG 000191/SQLite 000112 after rechecking migration tips. RunView plan reserves 000192/000113 and must not overlap.

**Global Constraints:** mechanical_worker owns only four new files `migrations/versioned/000191_craft_charge_start_journal.{up,down}.sql` and `migrations/sqlite/000112_craft_charge_start_journal.{up,down}.sql` in integration. No existing migration, Go service/repository, Run status, container or UI edits, commits or subagents. Other agents share Worktree; preserve them. New SQL may be ignored, so save full contents/hashes and report intent-to-add need.

**Review Focus:** primary key `(tenant_id,run_id,activity_key)`; distinct unique `(tenant_id,reservation_key)`; immutable Run/tenant scope, grant_id, call_id and run_revision; states limited to `intent`, `started`, `unknown`, `definitely_unstarted`. A recoverer must efficiently scan unresolved states. No TTL or migration default may silently convert unknown/intent to permission to start or release a hold. Keep PG/SQLite up/down reversible and aligned. The coordinator will separately enforce transitions and G4 transaction ordering.

## Task 1 — four SQL files

**Depends on:** reviewed T19 fix2 journal proposal. **Produces:** scoped journal table and unresolved scan index.

1. RED: In disposable SQLite at preceding migration tip, prove the journal table is absent and existing `agent_runs`/commercial reservations remain readable.
2. GREEN: Create the two up migrations with tenant/run/activity key (bounded text/varchar), grant/call/reservation identities, state check constraint, revision and timestamps; add tenant/run FK to `agent_runs` when both dialects can enforce it consistently. Add unique reservation identity and an index `(state,updated_at)` or equivalent for unresolved scans. Down drops only this table/index.
3. Verify SQLite apply, accepted intent insert, duplicate key rejection, invalid state rejection, unresolved scan, rollback and reapply without changing preexisting Run/reservation rows. Check PostgreSQL syntax manually or execute if local PG available. Run whitespace check and capture exact HEAD/status, four full SQL contents/hashes and report.

**Failure handling:** If cross-dialect FK or check syntax differs, adapt per dialect with the same effective invariant and disclose the exact difference; never omit tenant scope or permit an invalid state to make tests pass.
