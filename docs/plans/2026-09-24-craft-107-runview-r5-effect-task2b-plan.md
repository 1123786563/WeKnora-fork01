# Craft #107 R5 Effect Task 2b Transition Fence Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every Run lease, status, cancellation, decision and session-slot transition honor unresolved RunView effect intents under the same authoritative Run lock.

**Architecture:** `BeginEffect` already inserts `pending/unknown` under the Run transition row lock. Each transition that revokes the writer, changes epoch/status, or releases/transfers the session slot must acquire that lock and query `hasUnresolvedCraftRunViewEffects` before mutating. Transition-first denies a later effect; claim-first leaves the Run/slot fenced for exact reconciliation, with no new generation. Existing budget charge-start reconciling semantics may be reused only when their event/state meanings remain accurate; do not silently mark a provider effect resolved.

**Tech Stack:** Go/Gorm, current SQLite and isolated PostgreSQL effect schema, deterministic transactional barrier tests.

**Spec:** Approved Craft #107 T01/R5; `2026-09-24-craft-107-runview-r5-fence-plan.md` Task2; R5 Task2a Fix1 independently reviewed PASS; ADR-0008. R4 capture Task2 Fix1 is independently reviewed PASS and owns no lifecycle source during this Task.

## Global Constraints

- No physical Docker/OpenCode calls, DI/routing, migration edits or production gate enablement. Unknown/pending remains unresolved and blocks replacement Run generation and session writer transfer.
- Guard and transition share the exact Run row lock/transaction. A separate preflight Get/count is not authority; no DB transaction may encompass an external effect.
- Preserve R4's terminal capture enqueue and admission fence. A terminal transition may be delayed by unresolved effects; when later resolved, terminal enqueue still must occur exactly once. Preserve budget charge-start behavior without treating effect intents as budget settlements.
- No commit/push/stash. Record exact shared-file preimage/patch/postimage and concurrent base changes.

## Review Focus

- Claim-first vs transition-first barriers for CancelRun, owned cancel, session deletion, waiting_user pause, succeeded/failed SetStatus, event-driven success, decision cancel, ClaimDriver lease recovery and admission slot transfer.
- Pending and unknown each hold the existing writer slot and deny new claim/generation; after an exact finished outcome, the intended transition can proceed once without duplicate terminal/cancel events.
- No path clears `active_agent_run_id` or increments epoch while a RunView effect is unresolved, including session tombstone cleanup.

### Task 1: Transition inventory and locked guards

**Depends on:** R5 Task2a Fix1 independent PASS, R4 capture Task2 Fix1 independent PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/application/repository/agent_run.go`, `agent_run_lifecycle.go`, `agent_run_events.go`, `agent_run_decisions.go`, focused `_test.go` files for those transitions; `craft_run_view_effect.go` may be read but should remain unchanged absent an exact interface need. No R4 capture source, T19 or container files.

**Consumes / produces:** Existing `hasUnresolvedCraftRunViewEffects(tx,key)` and Run transition lock; produces guarded transition behavior and explicit unresolved outcome.

- [ ] Capture preimage. Enumerate every code path mutating `agent_runs.status/lease_owner/epoch` or `sessions.active_agent_run_id`; save the inventory in report. RED deterministic SQLite barrier tests for claim-first/transition-first on each named path, including pending and unknown, lease expiry, concurrent cancel and terminal capture enqueue.
- [ ] Add one narrow shared locked guard helper and call it in each transition immediately after lock and before first revocation/slot mutation. Do not convert an unresolved effect to success or cancel its budget hold. Ensure `ClaimDriver` and `DeleteSessionRuns` cannot bypass guard via existing charge-only reconciling/slot logic. Keep errors typed so callers know reconciliation is needed.
- [ ] Run focused repository SQLite and isolated PG transition tests/race; validate R4 capture terminal focused selector, formatting/diff. Save exact checkpoint/report/patch with shared file hashes.
- [ ] Independent reviewer checks inventory completeness, barrier semantics, R4 capture compatibility and separate Spec/quality verdict. Any missed path blocks Task3 physical wiring.

**Acceptance / failure handling:** The durable effect claim and all Run writer transitions are mutually exclusive under one lock; unresolved intents remain fenced for reconciliation. Full T01 remains gated by Task3 physical routing and R4 runtime quiescence.
