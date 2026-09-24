# T01 Admission Fence Migration Plan

> **For Codex:** Execute this narrow Superpowers plan with RED migration test, GREEN SQL, rollback verification, exact uncommitted checkpoint and independent Review. Do not commit.

**Goal:** Add the durable fields needed for T01/#120's abandoned input-admission claim recovery. This migration alone does not change RunStore or make recovery safe; it is a prerequisite for the service fix and T01 verification.

**Sources:** Approved Spec #107; #120 snapshot; `docs/plans/2026-09-23-craft-107-t01-fix1-review.md` in integration; T01 fix2 report in T01 Worktree. Current migration tip is PG 000189 / SQLite 000110; use PG 000190 and SQLite 000111 only after rechecking tips.

**Global Constraints:** mechanical_worker exclusive ownership of four new files `migrations/versioned/000190_craft_input_admission_fence.{up,down}.sql` and `migrations/sqlite/000111_craft_input_admission_fence.{up,down}.sql` in integration Worktree. No edit to prior migrations, application code, other SQL, commits, or subagents. Other agents share the repository; preserve their changes. These migration paths may be Git-ignored; checkpoint their full contents and hashes, and state the ignored status explicitly.

**Review Focus:** Existing `craft_session_requests` rows for every purpose remain readable. New fields are specific to `purpose='input_admission'` in application use; legacy rows have null/empty fields and must be treated conservatively until reconciled. No default value may fabricate an admitted Run. PG and SQLite up/down must be reversible without damaging preexisting columns/rows; no AutoMigrate.

## Task 1 — four SQL files

**Depends on:** reviewed T01 fix2 schema proposal. **Produces:** nullable `admission_run_id` (VARCHAR(64)), `admission_token` (VARCHAR(64)), `admission_state` (VARCHAR(16) NOT NULL DEFAULT ''), and nullable `lease_expires_at` (TIMESTAMP/DATETIME) on `craft_session_requests`; check or index for scoped claim lookup only if supported consistently by both dialects.

1. RED: Run a disposable SQLite migration-backed check demonstrating these columns are absent before the new migration and a legacy `input_decision` row survives.
2. GREEN: Implement PG/SQLite up migration and down migration in the four owned files, following existing versioned/sqlite conventions. Do not rewrite the old table. Keep null/empty legacy states fail-closed.
3. Verify SQLite apply, schema/legacy-row preservation, claim insert/lookup, rollback and reapply. Check PG syntax against local parser or live PG if available; clearly say if no PG execution. Run `git diff --check`, full content SHA-256 manifest, exact HEAD/status, and report.

**Failure handling:** If SQLite cannot safely drop columns on rollback under the supported engine, use a tested table rebuild preserving every original row and index; do not drop data. Report any version-number conflict before editing.
