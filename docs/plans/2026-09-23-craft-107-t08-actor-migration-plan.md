# T08 Actor Column Migration Plan

> **For Codex:** Narrow SDD RED → GREEN task with full ignored-file checkpoint and independent Review. No commit.

**Goal:** Add the nullable immutable initiating actor column for Run rows as a prerequisite to the T08 actor-principal fix. Preserve existing owner_id as the Task storage identity and leave legacy actor unknown.

**Sources:** Approved Spec #107/#126, `t08-content-acl-review.md`, `t08-actor-principal-design.md`, `t08-actor-principal-plan.md` Task 1, current agent_runs schema. Reserved unused numbers PG `000194_craft_run_actor`, SQLite `000115_craft_run_actor`; verify against current files before writing.

**Global Constraints:** mechanical_worker owns only four new migration files (up/down for each dialect) and its report. Do not edit repository/service/contracts/tests outside a disposable migration test script/report, existing migrations, T19/RunView files, commits or subagents. Integration Worktree is shared; other agents edit T19 Run repo and T05 knowledge. Migration files are Git-ignored: report exact full contents and SHA-256; controller will mark intent-to-add after independent review. The migration alone does not authorize collaborator execution.

**Review Focus:** `actor_user_id` nullable for legacy rows, no automatic backfill to owner_id, type/length parity with existing owner/user IDs, no destructive rewrite, up/down/reapply and old-row preservation. New Craft runs will be required to set actor by repository code in a later task. No index required unless a query uses actor in a new lookup.

## Task 1 — additive actor column

1. RED: disposable SQLite and PostgreSQL schema checks assert column absent before migration and old rows preserved after; record baseline.
2. GREEN: add actor column through additive ALTER TABLE. Down removes only that column safely; do not drop actor-bearing production data outside disposable test. Migration order follows 000193/000114.
3. Run SQLite up/old row/down/reapply and PG17 in disposable rolled-back schema if available, `git diff --check`, full content/hash checkpoint/report. Independent migration Review before actor repository Task consumes it.
