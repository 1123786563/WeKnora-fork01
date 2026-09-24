# T19 sandbox Task 4b Task 1 — fix 1 report

**Status:** Ready for independent re-review. All three scoped findings were addressed. No provider adapter or caller was enabled, and no commit/staging occurred.

## Scope and changes

Owned source files only:

- `internal/modules/execution/sandbox/remote_operation.go`
- `internal/modules/execution/sandbox/remote_operation_test.go`

The pre-fix hashes were recorded in the independent review: `remote_operation.go` `6fb550c24ff2a660ca21765b3821df5f860e74aebb723330d9ca53cc813524b0`; `remote_operation_test.go` `4c182f6a7ebb49a5b3cab29106a141a4f0ea2ada17b65232a80d5affc267d142`.

The fix validates the exec handle is non-nil, has a non-empty sandbox ID, and belongs to the client provider before invoking `StartExec`. Every error returned after invoking `StartExec` or `StartCreate` is now wrapped as `RemoteOperationUnknown`, preserving the underlying error and any receipt returned by the adapter. A malformed post-call receipt is reported as both `ErrRemoteOperationInvalidReceipt` and unknown, with the received receipt retained in the returned value and typed error. Only pre-dispatch validation errors remain definitely-not-started.

Tests now cover nil/empty/foreign exec targets with zero provider calls; missing and malformed exec/create receipts; raw timeout, cancellation and transport errors; typed unknown; and preservation of receipts returned alongside ambiguous errors. No test or implementation invokes the synchronous fallback.

## TDD and verification

RED (after adding the review regression cases, before implementation changes):

```text
go test ./internal/modules/execution/sandbox -run '^TestRemoteOperation' -count=1
FAIL: malformed receipts lacked unknown classification; raw post-dispatch errors escaped unclassified; invalid exec targets invoked StartExec (3 calls instead of 0).
```

GREEN:

```text
go test ./internal/modules/execution/sandbox -run '^TestRemoteOperation' -count=1
ok github.com/Tencent/WeKnora/internal/modules/execution/sandbox 1.055s

go test ./internal/modules/execution/sandbox -count=1
ok github.com/Tencent/WeKnora/internal/modules/execution/sandbox 6.860s

gofmt -d internal/modules/execution/sandbox/remote_operation.go internal/modules/execution/sandbox/remote_operation_test.go
PASS (no output)

git diff --check
PASS (exit 0)
```

## Exact checkpoint

```text
internal/modules/execution/sandbox/remote_operation.go  a6c678529485e9eb5a11ff16ad71c5a9b17ee3f25b48411386f8b90ddb7d99a3
internal/modules/execution/sandbox/remote_operation_test.go  7fcb99d0aa050f19d4602daabb0d77da43e49f241f5cec9914cf04097fee5462
docs/plans/2026-09-23-craft-107-t19-sandbox-task4b-fix1-task-local.patch  96ab5f1f3149a6297aaa6079c73ad0e202bbe56ffecb85b7cfe297be52ea72ed
```

The task-local patch is `docs/plans/2026-09-23-craft-107-t19-sandbox-task4b-fix1-task-local.patch`. Since the two source files are untracked in this shared worktree, the patch stores their complete `/dev/null` addition form; the pre-fix file hashes above provide the exact prior checkpoint identity.

## Remaining limits

The initiation contracts remain optional and no adapter implements them. This checkpoint addresses safe classification at the provider-neutral helper boundary; it does not prove any provider's receipt durability, same-key replay protection, authoritative observation, or live charge integration. Adapter work remains gated on independent re-review.
