# T19 Durable Charge-Start Coordinator Plan

> **For Codex:** Execute with Superpowers SDD, RED → GREEN → REFACTOR, exact per-task checkpoints, independent Spec/quality Review, backend validation, then whole-range OCR. No commit without authorization.

**Goal:** Close #138/F3: every chargeable model or sandbox start has a committed tenant/Run/activity intent and G4 hold before external initiation; budget pause, retry, cancellation and worker recovery cannot bypass an unresolved start.

**Sources:** Approved Spec #107, Issue #138, `CONTEXT.md`, ADR-0004, `2026-09-23-craft-107-dag.md`, `2026-09-23-craft-107-t19-fix2-review.md`, T19 Worktree `2026-09-23-craft-107-t19-central-seam-map.md`, reviewed PG 000191/SQLite 000112 journal migrations. Original BASE `4bcad69baf033a1310b4dce1372c8153e66adc81`; integration HEAD and content hash must be recorded immediately before each task.

**Global Constraints:** Use backend_implementer for each implementation task, distinct file ownership per dispatch, serial order for shared Run/budget state and database fixtures. All agents share a mutable integration Worktree; preserve unrelated changes and do not spawn agents. Do not infer provider idempotency or success from timeout. `intent` and its G4 hold remain unresolved across crash until explicit reconciliation. Never call an external provider while a database transaction is open. Real external initiation happens only after the intent/hold commit. A response/body-read failure after possible send is `unknown`. Every production bypass and Run status writer in the seam map must be addressed before marking T19 verified. Where model or sandbox provider lacks a stable activity ID or reconciliation method, fail closed and report the concrete blocked path, not an apparent success.

**Review Focus:** Commit-before-send invariant, same-key at-most-one initiation, durable unknown/hold, race ordering against pause and lifecycle writers, tenant/Run scope, stale worker/epoch fencing, live gateway and sandbox call sites, G4 conservation. PostgreSQL and SQLite behavior need separate evidence. Any content changed after a reviewed checkpoint requires relevant re-review.

## Task 1 — Journal-backed start coordinator

**Depends on:** reviewed 000191/000112 migrations. **Role:** backend_implementer; independent reviewer + backend_validator. **Owned files:** `internal/application/service/craft_budget.go`, new focused `craft_budget_start_test.go`, existing `internal/application/service/craft_budget_t19_test.go` solely to update the old B-first callback-inside-transaction assertion, `internal/modules/commercial/repository/commercial/budget_reservation.go` and its focused tests. **Consumes:** typed grant, Run lease/epoch, stable server activity key, G4 budget port. **Produces:** an API that first commits a journal `intent` and dispatched hold, then invokes a bounded transport start outside SQL, then durably records `started`, `unknown`, or trustworthy `definitely_unstarted`.

1. RED: inject a failure after external acceptance and a later result-write failure; prove an `intent` and G4 hold already survive. Same-key replay must not invoke the callback. Add controlled pause-first and start-first two-connection tests and a callback that ignores cancellation; PostgreSQL variant must run when DSN is available and explicitly report skip otherwise.
2. GREEN: split `StartBinding` into prepare transaction, initiation, and outcome transaction. Prepare locks the Run, checks live grant/lease/epoch and activity uniqueness, reserves G4 within the same transaction as the journal intent, and commits before returning permission to send. The start callback receives only the committed authorization and a bounded transport context. Outcome update uses compare-and-swap on intent state. On process/SQL/transport ambiguity retain `unknown` or unresolved `intent` and hold. Only a proven pre-send failure may be marked `definitely_unstarted` and released by explicit policy.
3. REFACTOR: remove the old callback-inside-transaction path and expose a narrow coordinator interface. Focused race/fault tests, full service/commercial packages, `git diff --check`, full-content checkpoint and independent review. Do not declare F3 complete; no production caller yet.

## Task 2 — Journal-aware Run transitions

**Depends on:** Task 1 reviewed and integrated. **Role:** backend_implementer; independent reviewer + backend_validator. **Owned files:** `internal/application/repository/agent_run.go`, `agent_run_events.go`, `agent_run_decisions.go`, `agent_run_lifecycle.go`, focused tests, and `internal/modules/workbench/service/workbench/interaction.go` only for its direct cancel writer. **Consumes:** journal unresolved query/transition contract. **Produces:** pause-requested semantics and one Run transition fence across claim/recovery, waiting, finalization, retry, cancel and bulk delete.

1. RED: test unresolved start while each production writer in seam map attempts a status/revision transition; no path may release a hold, start a new worker or turn pause-requested into resumed chargeability. Include stale lease/epoch and cross-worker tests.
2. GREEN: serialize transition on the tenant/Run row and unresolved journal in the same transaction; persist pause intent when an external start has begun and finalize pause only when the start resolves according to policy. Retain journal/hold on unknown even if Run becomes terminal. Route direct workbench cancel through the same contract.
3. Verify focused repository/workbench suites and SQLite/PostgreSQL races, exact checkpoint and review.

## Task 3 — Model gateway initiation

**Depends on:** Tasks 1–2 reviewed/integrated. **Role:** backend_implementer; independent reviewer + backend_validator. **Owned files:** `internal/handler/craft_model_gateway.go`, `internal/container/craft_model_gateway.go`, focused tests. **Consumes:** server-authenticated stable request/activity identity, typed coordinator, managed credential provider. **Produces:** no unfenced model `Client.Do` path.

1. RED: transport spy counts sends across same-key retries, pause races, pre-send failure, lost response and body-read failure. Assert at most one initiated send per activity and committed hold before any send.
2. GREEN: resolve request/credential before start, call coordinator exactly at bounded `Do` initiation, keep stream/body and usage after initiation outside SQL. Remove old `AuthorizeBinding` authority to send. Classify post-send errors as unknown. If upstream lacks dedupe/reconciliation, never retry an unresolved key.
3. Verify real container assembly, gateway tests, exact checkpoint and review.

## Task 4 — Sandbox/tool initiation

**Depends on:** Tasks 1–2 reviewed/integrated. **Role:** backend_implementer; independent reviewer + backend_validator. **Owned files:** `internal/modules/agentruntime/agent/runtime/tool_executor.go`, `internal/modules/execution/sandbox/session_manager.go`, `internal/modules/agentruntime/agent/tools/shell_exec.go`, `sandbox_write.go`, `sandbox_edit.go`, focused tests; **ownership amendment 2026-09-23:** `internal/application/service/agent_run_graph.go` and focused `agent_run_graph_test.go` only to inject the real committed-start coordinator through the production ToolExecutor/SessionBoundManager path. T08 actor Task 3 waits this checkpoint/review before changing graph. Obtain a new amendment before any other file. **Consumes:** persisted ToolPlan CallID/attempt and Run fence from runtime context. **Produces:** charge-start coordinator at each approved remote `Create`/`Exec` initiation, with bootstrap/maintenance/read/delete operations explicitly classified.

1. RED: same-attempt replay, pause-first/start-first, remote accepted/response-lost, stale epoch, and nonchargeable maintenance cases against a transport spy.
2. GREEN: propagate opaque activity/fence context from journaled tool attempt to the actual remote start. Wrap only provider initiation, not script execution or output wait. Mark ambiguous transport error unknown; block duplicate unresolved attempt.
3. Verify runtime/sandbox integration, no bypassing production path, exact checkpoint and review. This Task may run in a separate Worktree from Task 3 only after the coordinator interface is stable and owned files/state/resources are disjoint.

## Task 5 — Joined acceptance and full-range review

**Depends on:** Tasks 1–4 reviewed/integrated and T08 authority. **Role:** backend_validator + reviewer; no production edits. Drive live authenticated Task Budget/model/sandbox runs with SQLite and PostgreSQL where configured. Prove A-first/B-first ordering, crash before/after send, hold conservation, once attribution, committed budget pause, failed resume/retry while unknown, extension and deadline behavior, and fresh worker recovery. Reconcile any valid finding through a numbered SDD fix plan. Then run OCR against original BASE..HEAD and all uncommitted/staged/untracked delivery content, save reports and final checksums. T19 only becomes verified when no valid medium-or-higher issue remains and all production call sites and acceptance checks pass.
