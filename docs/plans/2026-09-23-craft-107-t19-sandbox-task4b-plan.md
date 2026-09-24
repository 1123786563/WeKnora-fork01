# T19 sandbox Task 4b: chargeable initiation capability

> **For Codex:** Execute this bounded plan with Superpowers SDD, TDD and an independent read-only review. This is a prerequisite, not T19 acceptance.

**Goal:** expose a provider-neutral operation identity and bounded initiation seam without calling the existing synchronous `Create`/`Exec` from a charge-start callback.

**Source:** approved Craft Spec #107, T19/#138, `t19-sandbox-initiation-task4a-report.md`, `t19-sandbox-sdk-research.md`, the T19 journal and budget-pause reviews. Worktree: `craft-107-integration`, BASE `4bcad69baf033a1310b4dce1372c8153e66adc81`. No commits authorized; save an exact uncommitted checkpoint and report.

## Global Constraints

- Existing `RemoteSandboxClient.Create/Exec` remain synchronous and retain their current behavior. Do not call either inside `StartBinding`.
- An ambiguous provider call is `unknown`, not `definitely not started`. The same operation key must never cause a second physical provider operation after ambiguity.
- Provider IDs are receipts, not proof of an idempotency-key contract. Persist or provide a durable receipt/lookup boundary before any replay. Unsupported providers fail closed.
- Work only in `internal/modules/execution/sandbox/remote_client.go`, new focused capability/test files, and Docker/E2B/Cube adapter files if contract tests establish the protocol. Do not change graph, budget, repository, RunView, or container assembly files in this task. Other agents own those surfaces. Do not revert their edits.

## Review Focus

Examine initiation versus completion timing; the moment a provider might have accepted; durable receipt recovery after response loss/restart; same-key replay; cancellation classification; unsupported Cube/create paths; and whether tests observe the provider call count rather than only a return value.

## Task 1 — capability contract and fail-closed defaults

**Depends on:** T19 journal and budget-pause service scoped PASS; SDK research. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `remote_client.go` plus new `remote_operation*.go` and focused tests in `internal/modules/execution/sandbox/`.

**Consumes:** `RemoteSandboxHandle`, `RemoteExecRequest`, `RemoteCreateRequest`, provider/operation key. **Produces:** explicit `StartExec/WaitExec/ObserveExec` and `StartCreate/WaitCreate/ObserveCreate` optional capabilities or equivalent typed seams; provider operation ref and observation types; typed unsupported/unknown results.

1. Write RED contract tests: synchronous-only fake must not advertise initiation; unsupported Cube/create path never calls old `Create/Exec`; unknown start does not cause retry; returned receipt preserves provider/sandbox/op key.
2. Add the smallest typed contracts; leave existing interface behavior intact. Do not advertise a capability until an adapter implements all start, wait and observe properties.
3. Run focused package tests and `git diff --check`; save exact source/test hash checkpoint, test output and report. If a provider API cannot supply durable receipt or authoritative lookup, record evidence and leave it unsupported.

**Acceptance:** the interface cannot accidentally route a chargeable call through synchronous full-operation methods; no provider marked supported without start/observe proof. **Failure handling:** preserve fail-closed status and report the SDK limitation; do not fabricate provider identity.

## Task 2 — provider adapters, conditional on Task 1 review

**Depends on:** Task 1 independent Spec/quality PASS. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** Docker/E2B adapter and focused tests only.

Implement one provider at a time only if tests prove bounded start, stable receipt and recovery. E2B `Commands.Start`/PID/Connect/List and Docker ExecCreate/ExecStart/Inspect are candidates; provider-create and Cube exec are unsupported under the current public SDK evidence. Test same-key retry and ambiguous response with zero second starts. Save another checkpoint and independent review package. Production wiring to `StartBinding` is a later task after durable receipt storage is specified and reviewed.

**Expected verification:** focused package/race tests where practical; exact source hashes. **Acceptance:** real provider calls are confined to the proven capability; unsupported operations fail closed. **Failure handling:** stop adapter work and report the missing provider contract rather than substituting synchronous `Exec/Create`.
