# Craft #107 T19 S3 Fix2 — Task 1 report

**Base HEAD:** `a5e9195acd6500c085c85d60c852148e7bbbbf34`.  
**Owned files:** `internal/application/service/craft_docker_restricted_exec.go` and `_test.go` only.

## Review corrections

After each successful `Observe`, `Wait` now rechecks its bounded context before evaluating terminal state. A terminal inspect that returns after cancellation/deadline therefore stays `unknown`, includes the exact durable receipt, and does not alter the already claimed journal row.

The added first-inspect false/zero liveness test returns `Unknown` with exit code zero but no prior running provenance. Wait continues to its bound and remains unknown; it does not infer success. The existing adapter test covers the real mapping from Docker `Running=false, ExitCode=0` without positive running evidence to unknown. No duration was fabricated: `DurationAvailable` remains false. Task 2 is untouched.

## RED → GREEN

RED was run with only the post-Observe context check temporarily removed. The deterministic fake first returned `Running`, then blocked the second inspect while ignoring cancellation. Releasing it with terminal success after cancellation and terminal failure after Wait deadline failed as expected: both branches returned nil error instead of `RemoteOperationUnknown`.

Restoring the context fence made both branches pass. The false/zero liveness test also passes and asserts at least one inspect, no prior running evidence, unknown outcome and retained durable claim.

## Verification

```text
$ go test ./internal/application/service -run '^TestCraftDockerRestrictedWait(CancellationDuringInspectRejectsLateTerminal|FirstFalseZeroRemainsUnknown)$' -count=1
ok   github.com/Tencent/WeKnora/internal/application/service 2.409s
$ go test ./internal/application/service -run '^TestCraftDockerRestricted' -count=1
ok   github.com/Tencent/WeKnora/internal/application/service 7.549s
$ go test -race ./internal/application/service -run '^TestCraftDockerRestricted' -count=1
ok   github.com/Tencent/WeKnora/internal/application/service 8.033s
$ go test ./internal/application/service -run '^$' -count=1
ok   github.com/Tencent/WeKnora/internal/application/service 1.029s [no tests to run]
$ gofmt -d <two owned files>
(no output)
$ git diff --check
(no output; exit 0)
```

The owned files are untracked in the integration Worktree, so a direct source check also confirmed no trailing whitespace. The deterministic RED command used `go test ./internal/application/service -run '^TestCraftDockerRestrictedWaitCancellationDuringInspectRejectsLateTerminal$' -count=1`; it failed both subtests with `Expected error with "remote operation outcome unknown" in chain but got nil` before the post-Observe context fence was restored.

## Checkpoint

Preimage hashes are in `2026-09-24-craft-107-t19-docker-s3-fix2-task1-pre.json`. The task-local patch, postimage archive and final SHA-256 values are listed in `2026-09-24-craft-107-t19-docker-s3-fix2-task1-checkpoint.json`.

## Remaining gates

This closes the bounded Wait terminal-after-cancellation/deadline race for the tested ordering and documents the first-inspect false/zero liveness limit. Authoritative execution duration remains unavailable and S3-R1 stays open. Coordinator-to-physical-start proof remains assigned to the separate Task 2. No production routing or commit was added.
