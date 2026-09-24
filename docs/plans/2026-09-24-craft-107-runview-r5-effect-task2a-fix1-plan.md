# Craft #107 R5 Effect Task 2a Fix 1 Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close independent Review R5E-1 and R5E-2 before any physical provider route can use effect authority.

**Architecture:** In `BeginEffect`'s current Run transaction, require the exact completed fenced allocation intent for the same tenant/Run/generation and admission identity before replay or insertion of a provider intent. A provider `FinishEffect(succeeded)` requires a nonempty observable receipt; absent identity remains unresolved.

**Tech Stack:** Go/Gorm, existing effect authority store, SQLite/isolated PostgreSQL focused tests.

**Spec:** Approved Craft #107 T01/R5; `2026-09-24-craft-107-runview-r5-effect-task2a-plan.md`; independent `2026-09-24-craft-107-runview-r5-effect-task2a-review.md`.

## Global Constraints

- No physical provider calls, transition edits, DI/routing or migration changes. Keep production default-off.
- Maintain exact current Run, actor, scope, fence, lease and snapshot digest checks. A finished allocation may replay after a valid new fence, but a provider intent remains exact-fence and never grants a second send.
- No commit/push/shared stash; capture exact task-local pre/post hashes and patch. Another agent owns `craft_docker_normal_input.go`; its transient compile state must not be attributed to this fix.

## Review Focus

- Legacy key-only `Allocate` followed by direct `BeginEffect` never returns `maySend=true` or inserts an effect.
- Exact allocation row must be `finished/succeeded`, receipt equal generation and bound to the admitted identity under one lock; missing/pending/unknown/foreign row fails.
- `FinishEffect(succeeded)` with empty identity leaves intent unresolved. Unknown stays unknown; successful replay with changed receipt conflicts.

### Task 1: Allocation prerequisite and nonempty success receipt

**Depends on:** R5 Task2a checkpoint and independent FAIL review. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/application/repository/craft_run_view_effect.go` and focused `_test.go` only; no migration or other shared files.

**Consumes / produces:** Current Run/effect transaction and immutable allocation intent; produces claim authorization only for a completed admitted allocation and durable success only with observed identity.

- [ ] Capture preimage and RED focused tests for legacy direct Begin, incomplete/foreign allocation intent, empty-success and retained unresolved state.
- [ ] Implement common exact allocation validation under the same Run transition lock and require nonempty receipt for success; preserve exact replay/unknown behavior.
- [ ] Run focused SQLite and isolated PG tests, migration tests only if touched (not expected), race selector and diff check. Save checkpoint, patch, report with commands and results.
- [ ] Independent reviewer issues separate Spec/quality verdict and exact hash coverage. Task2b transition work waits PASS.

**Acceptance / failure handling:** No provider send permission without a fenced completed allocation; no unidentifiable success can release unresolved Run fencing.
