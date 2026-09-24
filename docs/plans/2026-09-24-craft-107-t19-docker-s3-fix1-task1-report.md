# Craft #107 T19 S3 Fix1 — Task 1 report

**Base:** `a5e9195acd6500c085c85d60c852148e7bbbbf34` in the integration Worktree.  
**Scope:** `internal/modules/execution/sandbox/docker_restricted_exec.go`, `_test.go`; `internal/application/service/craft_docker_restricted_exec.go`, `_test.go`. Task 2/coordinator proof is not included.

## Implementation

Added `CraftDockerRestrictedExec.Wait`, which first loads the durable claim and exact receipt, then polls `Observe` for only that receipt until it sees a terminal state or the caller's `maxWait`/context ends. It returns terminal state and exit code while keeping stdout/stderr explicitly unavailable. Inspect failure, deadline, cancellation, and unknown observations do not release the claim; ambiguous wait errors carry `RemoteOperationUnknown` and the receipt when available.

Added a context cancellation check immediately after successful durable `Claim`, before consuming the in-process permission or calling `StartOutputlessExec`. Cancellation returns unknown and leaves the durable `intent`/claim intact.

The pinned Docker ExecInspect response exposes `Running`, `ExitCode`, and identity, but no execution timestamps. The result therefore carries `DurationAvailable=false`; poll elapsed time is not fabricated as command duration. The bounded Wait and cancellation fence are implemented, but the plan explicitly says to leave R1 open when authoritative duration is unavailable.

## RED → GREEN

- RED for post-claim cancellation: temporarily removed only the `ctx.Err()` fence and ran `go test ./internal/application/service -run '^TestCraftDockerRestrictedCancellationAfterClaimSkipsProviderStart$' -count=1`. It failed as expected: `Expected error with "context canceled" in chain but got nil`. Restored the fence and reran the same test: `ok github.com/Tencent/WeKnora/internal/application/service 1.842s`.
- The first Wait test compile RED was `CraftDockerRestrictedWait` undefined and missing sequence fields on the fake. After adding the method and fake sequence support, `go test ./internal/application/service -run '^TestCraftDockerRestricted(Wait|CancellationAfterClaim)' -count=1` returned `ok github.com/Tencent/WeKnora/internal/application/service 3.820s`.
- RED for Wait: before adding the service method/result shape, `go test ./internal/application/service -run '^TestCraftDockerRestricted(CancellationAfterClaim|Wait)' -count=1` failed to compile because `CraftDockerRestrictedExec.Wait` and test fake observation sequencing were absent. Implemented Wait and the sequence-aware test fake; focused tests then passed.

Coverage added for post-claim cancellation with zero provider starts and retained durable claim; fast and slow terminal states with exact receipt and running provenance; canceled wait; deadline; inspect error; and explicit output/duration availability. Existing adapter tests retain false/zero-without-positive-running and inspect-error checks.

## Verification

```text
$ go test ./internal/modules/execution/sandbox -run '^TestDockerRestricted' -count=1
ok   github.com/Tencent/WeKnora/internal/modules/execution/sandbox 1.262s
$ go test ./internal/application/service -run '^TestCraftDockerRestricted' -count=1
ok   github.com/Tencent/WeKnora/internal/application/service 4.840s
$ go test -race ./internal/modules/execution/sandbox -run '^TestDockerRestricted' -count=1
ok   github.com/Tencent/WeKnora/internal/modules/execution/sandbox 2.739s
$ go test -race ./internal/application/service -run '^TestCraftDockerRestricted' -count=1
ok   github.com/Tencent/WeKnora/internal/application/service 13.802s
$ go test ./internal/modules/execution/sandbox ./internal/application/service -run '^$' -count=1
ok   github.com/Tencent/WeKnora/internal/modules/execution/sandbox 0.601s [no tests to run]
ok   github.com/Tencent/WeKnora/internal/application/service 1.019s [no tests to run]
$ gofmt -d <four scoped files>
(no output)
$ git diff --check
(no output; exit 0)
```

The scoped implementation/test files are untracked in this integration tree, so a direct content check also confirmed there is no trailing whitespace in those four files.

The disposable real-Docker test was not rerun for this Task 1 change. No Task 2 coordinator/physical-start proof is claimed. No production routing or commit was added.

## Checkpoint

Exact preimage manifest: `2026-09-24-craft-107-t19-docker-s3-fix1-task1-pre.json`. Task-local diff: `2026-09-24-craft-107-t19-docker-s3-fix1-task1.patch`. Postimage archive and owned-file hashes are in `2026-09-24-craft-107-t19-docker-s3-fix1-task1-checkpoint.json`.

## Remaining gate

S3-R3's post-claim cancellation path and the bounded same-receipt wait behavior are covered by the focused tests. S3-R1 remains open for an authoritative command execution duration because the pinned Docker API does not provide start/finish timestamps. S3-R2 remains open for Task 2's coordinator-to-physical-start proof.
