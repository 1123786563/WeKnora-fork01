# T63 Task5 Review Fix — serialize retirement with durable run admission

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Narrow repair of Task5 commit `cded1932c8fc8ffc8401d5ab883052f33c0d102b`.

**Goal:** Prevent retired local Agents from crossing the final durable run-admission boundary, preserve idempotent admitted-request replay, and map queue_next denial to HTTP 409.

**Findings:** F1 HIGH: `RetiredVariantAgentExists` is a separate read before `CreatePending`/`AgentRunStore.Admit`; retirement can commit between it and run insertion. F2 MEDIUM: gate precedes same-request reconciliation, so admitted retries after retirement are denied. F3 MEDIUM: queue_next command mapper omits `ErrAgentUseDenied`.

**Architecture ruling:** Make `AgentRunStore.Admit` the authoritative serialized boundary. Inside its existing transaction, after idempotency lookup and before slot/run insertion, perform the first variant operation as a write/no-op UPDATE locking all tenant+local_agent_id variant rows, then read states; deny if any matching row is retired. RetireVariant CAS touches same row, giving DB commit order. Keep early gate for quick rejection. Pass AgentID explicitly in `agentruntime.Admission`; do not infer security input from snapshot JSON. Reconcile a same-hash existing dispatching/admitted request before early gate. Pending retries still pass gate. A race denial happens after workbench pending intent/budget reservation exists; transaction must create no Run/slot/message durable admission. Mark that request rejected (durable intent record) and release only its unstarted reservation; the common early gate continues to leave no pending row. Do not delete immutable request history.

## Constraints / ownership

- Continue only in T63 worktree after Task5 checkpoint.
- Owned files: `internal/modules/agentruntime/agent/runtime/contracts.go`; `internal/application/repository/agent_run.go` and its test; `internal/modules/workbench/service/workbench/admission.go` and `admission_agent_use_test.go`; `internal/handler/session/workbench_commands.go` and relevant tests. Touch additional files only if a compile-required fake/interface needs the new AgentID field, and list them explicitly.
- No T64 files, no unrelated runtime redesign. Local commit authorized.

## Task 1 — authoritative gate and idempotent replay

1. RED: deterministic real SQLite tests force RetireVariant between early gate and final Admit and assert no AgentRun/session slot is created, pending request becomes rejected and reservation is released. Test opposite ordering (Admit commits first, retirement follows), replay an already-admitted same-hash request after retirement returns original Run, mismatched hash still conflicts, pending retry after retirement is denied, and queue_next error maps 409.
2. GREEN: add explicit AgentID to runtime Admission. In `AgentRunStore.Admit`, after existing request idempotency check, acquire write/no-op UPDATE on matching `(tenant_id, local_agent_id)` variant rows before reading `state`; reject retired state with a shared runtime sentinel. Map sentinel to `workbenchservice.ErrAgentUseDenied`; service alias/wrapping must preserve `errors.Is`. Reorder Start same-request reconciliation so an existing dispatching/admitted request resumes before early gate; keep gate before new CreatePending. On final-boundary denial, mark the pending intent rejected and release only its unstarted budget reservation. Add command mapper conflict mapping.
3. Use file-backed SQLite + separate connections/channel or callbacks, no sleeps; document that this proves SQLite serialization, not PostgreSQL execution. Preserve normal active/no-agent admission.
4. Run focused repo/workbench/handler tests, adjacent session/container filters, build, diff-check; commit and report exact evidence.

**Review focus:** A retired Agent never gets a new durable Run/session slot even in the gate→retire race; already admitted idempotent replay remains stable; no retry double-release; both Start and queue_next return 409; DB write lock precedes variant state read.
