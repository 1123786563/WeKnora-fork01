# T55 High Review Repair — Task 1 Report

## Result

Addressed the request-cancellation and settlement-error parts of T55-R1. Recovery settlement now uses a 5-second context detached from request cancellation. Settlement CAS failures are joined with the original provider error and returned; dispatcher-not-wired recovery is also settled through that path. A persisted `dispatched` claim can be reconciled through the resolve endpoint using provider reads only: an observed PR settles it to `delivered`; absent PR facts never release the claim to `pushed`.

## Durable claim limitation / interface gap

The existing Delivery row has only the state value `dispatched`; it has no claim owner/token, lease deadline, attempt generation or fencing version. Therefore no caller can distinguish a process that crashed before sending POST from an active process whose POST is still in flight. Moving a no-PR `dispatched` row to `pushed` based on elapsed time or an empty read could race the original request and permit a duplicate PR create. The implemented reconciliation can recover a durable claim when remote PR facts prove success. When PR is absent, it deliberately keeps `dispatched` and fails closed. Fully releasing abandoned no-PR claims requires a durable claim lease plus fencing/ownership semantics (and corresponding schema/interface decision); that is not safely derivable from current interfaces. No migration or speculative timeout was added.

## Changed files

- `internal/modules/codedelivery/service.go`
- `internal/modules/codedelivery/dispatcher.go`
- `internal/application/repository/delivery_recovery_http_test.go`

## Verification

Commands run from this worktree:

- `go test ./internal/application/repository/ -run '^TestT55(CanceledRecoverySettlesUnknownUsingIndependentContext|RecoverySettlementFailureIsSurfaced|DispatchedClaimReconcilesRemotePRWithoutWriting|DispatchedClaimWithoutPRFactRemainsFailClosed)$' -count=1` — PASS.
- `go test ./internal/modules/codedelivery/ -count=1` — PASS.
- `go test ./internal/application/repository/ -run 'TestT25|TestT55' -count=1` — PASS.
- `git diff --check` — PASS.

The tests verify cancellation after the PR request reaches the provider settles `unknown` and then reconciles by reads; a DB trigger that rejects the `unknown` CAS produces an error response and leaves the row visibly `dispatched`; a known PR reconciles a dispatched row with no writes; and an absent PR leaves that claim dispatched with no writes.

## Commit and review package

- Base: `36eef1593`
- Implementation commit: `f448958131f62b12701b74f05e6aaa35debc5c1d`
- Review package: `.superpowers/sdd/plan-t55/task-1-high-review-repair.patch`
- Review package SHA-256: `740fb9ceb297c10674b23ab271e28506f4c7e665e0bc7db18b902b5e6fe42fdc`

Task 2 was not started, per the parent instruction to pause for independent validation after Task 1. Task 1 remains incomplete only for safe automatic release of a no-PR abandoned claim; the exact missing durable interface is described above. No live credentials or real providers were used.
