# Craft RunView Binding Plan

> **For Codex:** Execute with Superpowers SDD, RED → GREEN → REFACTOR, exact checkpoint, independent Review and backend validation. Do not commit.

**Goal:** Persist one server-owned per-Run execution-view identity for T01/T05 isolation, recover it after restart, and reject cross-scope substitution. This is a prerequisite; no OpenCode/container filesystem isolation is claimed by a database row alone.

**Sources:** Approved Spec #107; #120/#124 snapshots; `docs/plans/2026-09-23-craft-107-per-run-isolation-design.md` in T01 Worktree; existing Craft Workspaces and `agent_runs` schema/repository conventions. PG 000190/SQLite 000111 are already used by T01 admission; reserve PG 000191/SQLite 000112 for T19 journal, so this Task owns PG 000192/SQLite 000113 only after rechecking current tips. Integration HEAD `a5e9195` plus uncommitted reviewed checkpoints.

**Global Constraints:** backend_implementer exclusive ownership of four new migration files `migrations/versioned/000192_craft_run_views.{up,down}.sql`, `migrations/sqlite/000113_craft_run_views.{up,down}.sql`, new `internal/application/repository/craft_run_view.go` and focused tests, plus a small typed internal contract file `internal/modules/craft/run_view.go` and its tests. No existing Craft runtime/service/container/OpenCode client edits, T19 journal DDL, unrelated migrations, commits or subagents. Others share integration Worktree; preserve their changes. New SQL may be ignored: save full content/hashes and state that it needs intent-to-add for OCR/delivery.

**Review Focus:** identity is `(tenant_id, owner_id, session_id, run_id)` with exact scope validation; `run_id` is admitted server identity, not caller-selected path. Generation is random server-owned and immutable. No arbitrary filesystem path from client/model is stored as authority. A retry returns the same generation/container/session binding; a conflicting Run/scope or concurrent allocator fails closed. An unresolved OpenCode session/container create must not silently replace the binding. Legacy Workspaces remain readable; row creation must not imply isolation enabled.

## Task 1 — schema and typed store

**Depends on:** existing admitted `agent_runs` schema; independent of OpenCode client-header Task. **Produces:** `craft_run_views` persisted binding with tenant/run primary key, owner/session identity, random generation, runtime/container/session identity, state and timestamps; `Allocate`, `Load`, `BindRuntime` compare-and-swap ports scoped to exact Run.

1. RED: Add SQLite migration test showing table absent before migration, then apply/rollback/reapply; add repository tests for retry-stable generation, cross-tenant/owner/session denial, two-connection concurrent allocation, runtime session CAS, restart reload, and unresolved state. Start with missing API/table failures.
2. GREEN: Add four reversible dialect migrations with scoped foreign key or equivalent tenant/run guard consistent with existing `agent_runs` schema and indexes for recovery scans. Implement a small typed store that reads authoritative admitted Run/session scope before allocation, generates a cryptographically random opaque generation, inserts once, and returns an immutable view. CAS-bind a concrete runtime/container/session only once (same-value replay allowed); mismatched rebind fails.
3. REFACTOR: Keep table and port small; avoid storing absolute host paths. Run focused repository tests, SQLite up/down/up, relevant Craft Go tests, `git diff --check`; manually inspect PG syntax or run local PG if available. Capture exact full-content checkpoint and report. State that container mount/event isolation and output collection are not implemented.

**Failure handling:** If the existing `agent_runs` constraints cannot be referenced portably in both dialects, implement an in-transaction tenant/session/owner read validation and report the FK difference; never allocate an unscoped view. Do not auto-recreate a lost runtime session for an unresolved Run.
