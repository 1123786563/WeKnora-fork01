# T19 sandbox Task 4b fix 1 — independent scoped re-review

**Scope:** Read-only re-review of `remote_operation.go` and `remote_operation_test.go` against the three findings in `t19-sandbox-task4b-task1-review.md`, the fix 1 plan, approved Craft Spec #107, T19 durable-start constraints, Task 4a evidence, and SDK research. This verdict covers the provider-neutral helper contract only; no provider adapter or production charge caller is in scope.

**Checkpoint identity:** SHA-256 `remote_operation.go` `a6c678529485e9eb5a11ff16ad71c5a9b17ee3f25b48411386f8b90ddb7d99a3`; `remote_operation_test.go` `7fcb99d0aa050f19d4602daabb0d77da43e49f241f5cec9914cf04097fee5462`; task-local patch `96ab5f1f3149a6297aaa6079c73ad0e202bbe56ffecb85b7cfe297be52ea72ed`. All match the worker report. `git apply --check --reverse` on the task-local patch passed against the current two untracked source files, confirming its complete-addition patch matches the checkpoint.

**Verification:** Focused `go test ./internal/modules/execution/sandbox -run '^TestRemoteOperation' -count=1` passed (1.245s). Full `go test ./internal/modules/execution/sandbox -count=1` passed (6.336s). `git diff --check` and `gofmt -d` on the two source files passed. These are local package checks, not live provider or T19 acceptance tests.

## Finding closure

1. **Prior High — post-send invalid receipt: CLOSED.** `StartRemoteExec` lines 114–120 and `StartRemoteCreate` lines 135–141 now return a `RemoteOperationError` with `State: unknown` after any invoked start that returns an invalid receipt. `Unwrap` preserves `ErrRemoteOperationInvalidReceipt`, and both the returned value and typed error retain the received ref. The tests exercise missing/mismatched exec and create receipts, unknown classification, and one physical call.

2. **Prior Medium — exec target provenance before invocation: CLOSED.** `StartRemoteExec` lines 104–109 reject an empty operation key and nil, empty-ID, or foreign-provider handle before calling the initiator. The call-count test at `remote_operation_test.go:111–130` confirms zero physical starts for those invalid targets. Post-call receipt provider, operation ID, key, and sandbox ID are checked at lines 118–120.

3. **Prior Medium — raw adapter errors escaping without ambiguity: CLOSED.** `StartRemoteExec` lines 114–116 and `StartRemoteCreate` lines 135–137 conservatively wrap every returned post-invocation error as unknown. `RemoteOperationError.Unwrap` preserves the original timeout, cancellation, or transport diagnostic. Tests at `remote_operation_test.go:156–185,205–243` cover raw errors for both operations and receipt retention when a start returns a ref with an error.

## Verdict and limits

**Scoped Spec compliance: PASS. Scoped code quality: PASS.** The prior three blocking findings are closed at this checkpoint. The old synchronous `Create`/`Exec` methods are never fallback paths, and no current provider advertises this optional initiation capability.

This pass does **not** prove a provider's bounded acceptance, durable receipt persistence, authoritative observation, same-key replay prevention after restart, or integration with `StartBinding`. Those remain Task 4b adapter and later T19 integration gates. In particular, this stateless helper classifies an ambiguous result but cannot itself prevent a caller from invoking it again with the same key; adapter and durable caller evidence is still required before enabling chargeable sandbox starts.
