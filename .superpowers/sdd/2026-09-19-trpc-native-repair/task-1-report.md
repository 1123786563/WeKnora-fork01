# Task 1 report — pinned tRPC candidate

## Scope

Changed only the root SDK pin/checksums, the native Runner probe, and P0 evidence/ledger files listed in the Task 1 brief. No product Runner wiring, local `replace`, Session/Memory backend selection, or recovery implementation was added.

## RED — v1.10.0

The probe first creates and reads a Session from the `inmemory.SessionService` passed into the real `runner.Run` path. After the completed tool turn, it asserts that `Events` is non-empty and that `after.UpdatedAt.After(before.UpdatedAt)` is true. A RED assertion requiring the pre-existing Session failed before that setup was added, proving the old “non-zero `UpdatedAt`” assertion could only observe a Session created by Runner and could not prove mutation of an existing one.

```sh
GOWORK=off go test -race ./internal/agent/nativeprobe -count=20 -v
```

It exited 1. The second native Runner iteration reported a data race between `trpc-agent-go@v1.10.0` `session.(*Session).Clone` and `session.(*Session).UpdateUserSession`, reached through the function-call state-delta snapshot and in-memory Runner `AppendEvent` path. This is the expected historical blocker and was not suppressed.

## GREEN — v1.11.0 candidate

Applied the exact root requirement `trpc.group/trpc-go/trpc-agent-go v1.11.0`, then ran `GOWORK=off go mod tidy`. The SDK requires the accompanying indirect `trpc-a2a-go` version update. There is no local absolute-path `replace` for either module and no production-source adjustment was required.

The following commands exited 0:

```sh
GOWORK=off go test ./internal/agent/nativeprobe -count=1 -v -timeout 45s
GOWORK=off go test -race ./internal/agent/nativeprobe -count=20 -v
GOWORK=off go test ./internal/agent/trpc -count=1 -v
```

The race gate ran all twenty native-probe iterations cleanly. The direct-consumer inventory was produced with:

```sh
GOWORK=off go list -deps -f '{{with .Module}}{{if eq .Path "github.com/Tencent/WeKnora"}}{{$.ImportPath}} {{join $.Imports " "}}{{end}}{{end}}' ./... | rg 'trpc\\.group/trpc-go/trpc-agent-go'
```

It identified `internal/agent/trpc`, `internal/application/service`, and `internal/agent/recoverytest/provider` (plus the native probe test consumer). `internal/agent/trpc` passed above.

## Existing direct-consumer failure and decision

The following commands were launched after the upgrade:

```sh
GOWORK=off go test ./internal/application/service -run TestExecuteDurableRunPersistsBudgetExhaustionForNotification -count=1 -v
cd /tmp/weknora-trpc-task1-baseline && GOWORK=off go test ./internal/application/service -run TestExecuteDurableRunPersistsBudgetExhaustionForNotification -count=1 -v
```

Both commands exited 1. In each worktree the test reports `graph execution: task_budget_exhausted` and observes only `run_started` and `run_failed`; the expected budget-exhaustion notification is absent. The exact `75523c9c9` baseline has the same failure, so it is not introduced by the v1.11.0 upgrade. It blocks complete direct-consumer acceptance. The repeated native race gate is green, but this existing failure, together with the unselected persistent Session/Memory backend, append-failure barrier, PostgreSQL, real Provider, recovery, and client gates, keeps P0 **NO-GO** for native Runner product execution.

## Remaining work

Repair and verify `TestExecuteDurableRunPersistsBudgetExhaustionForNotification` in its owning scope, then complete the separate P0 persistence/recovery/provider/PostgreSQL/client acceptance gates. This Task 1 does not authorize product cutover.
