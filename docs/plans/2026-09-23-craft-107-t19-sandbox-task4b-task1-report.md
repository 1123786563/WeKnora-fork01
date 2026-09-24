# T19 sandbox Task 4b, Task 1 report

**Status:** DONE_WITH_CONCERNS (contract implemented; no provider is enabled).
**Worktree:** `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
**Task scope:** capability contract and fail-closed defaults only. No provider adapters or production callers changed. No commit or staging performed.

## Evidence reviewed

- Task plan: `2026-09-23-craft-107-t19-sandbox-task4b-plan.md`, Task 1.
- Task 4a boundary evidence: `2026-09-23-craft-107-t19-sandbox-initiation-task4a-report.md`.
- SDK evidence: `2026-09-23-craft-107-t19-sandbox-sdk-research.md`.
- The SDK research documents synchronous-only Cube `Create`/`Commands.Run`; Cube remains unsupported for initiation. Docker and E2B candidates were intentionally left for Task 2 after this contract review.

## Changes

- Added optional complete `RemoteExecInitiator` and `RemoteCreateInitiator` contracts. A client only advertises initiation by implementing start, wait and observe methods together; the base synchronous `RemoteSandboxClient` remains unchanged.
- Added `RemoteOperationRef`, observation/state types, and a typed ambiguous `RemoteOperationError`.
- Added `StartRemoteExec` / `StartRemoteCreate` dispatch helpers. They reject empty keys, require optional capability assertion, validate returned provider operation identity, and never call the synchronous `Exec` / `Create` fallback.
- Added focused tests for synchronous-only fail-closed behavior, Cube create fail-closed behavior, unknown start being returned without helper retry, receipt preservation, and mismatched operation-key rejection.

## TDD and verification

RED was observed before adding the contract:

```text
go test ./internal/modules/execution/sandbox -run '^TestRemoteOperation' -count=1
FAIL: undefined StartRemoteExec, RemoteOperationRef, RemoteOperationObservation, and related contract symbols
```

GREEN and package regression checks:

```text
gofmt -w internal/modules/execution/sandbox/remote_operation.go internal/modules/execution/sandbox/remote_operation_test.go
go test ./internal/modules/execution/sandbox -run '^TestRemoteOperation' -count=1
ok github.com/Tencent/WeKnora/internal/modules/execution/sandbox 0.923s

go test ./internal/modules/execution/sandbox -count=1
ok github.com/Tencent/WeKnora/internal/modules/execution/sandbox 5.695s

git diff --check
PASS (exit 0)

gofmt -d internal/modules/execution/sandbox/remote_operation.go internal/modules/execution/sandbox/remote_operation_test.go
PASS (no output; both untracked Go files are formatted)
```

`git diff --check` checks tracked changes; because the new source/test files are untracked at this checkpoint, their whitespace/format check is recorded separately through `gofmt -d` above. The final task diff consists of those two new files and this report.

## Exact uncommitted checkpoint

Only the two new owned source/test files below are this Task 1's source changes. `remote_client.go` is unchanged at the hash shown. The worktree has unrelated concurrent changes; they were preserved and are outside this task.

```text
internal/modules/execution/sandbox/remote_client.go  7b8fc6c06a7934a985625ec12aba7cfcb9b9e7661d74ec0379d31c5baeb3b46d
internal/modules/execution/sandbox/remote_operation.go  6fb550c24ff2a660ca21765b3821df5f860e74aebb723330d9ca53cc813524b0
internal/modules/execution/sandbox/remote_operation_test.go  4c182f6a7ebb49a5b3cab29106a141a4f0ea2ada17b65232a80d5affc267d142
```

Report path: `docs/plans/2026-09-23-craft-107-t19-sandbox-task4b-task1-report.md`.

## Remaining limits

- No provider implements the new interfaces in this task. In particular, no claim is made about durable receipt storage, same-key idempotency, crash recovery, or authoritative provider observation. The interface comments require those properties before a provider can safely implement it.
- The unknown-start test verifies the dispatch helper makes exactly one capability call and preserves the unknown classification. Preventing a later caller from reissuing the same operation key remains the responsibility of a future adapter backed by durable receipt recovery or authoritative key lookup.
- No independent validator/reviewer is included in this worker checkpoint; Task 1's review remains outstanding.
