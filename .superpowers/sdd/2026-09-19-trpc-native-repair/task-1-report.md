# Task 1 report — pinned tRPC candidate

## Scope

Changed only the root SDK pin/checksums, the native Runner probe, and P0 evidence/ledger files listed in the Task 1 brief. No product Runner wiring, local `replace`, Session/Memory backend selection, or recovery implementation was added.

## RED — v1.10.0

The probe now retains the `inmemory.SessionService` passed into the real `runner.Run` path and asserts that the completed tool turn mutates the persisted Session (`Events` non-empty and `UpdatedAt` non-zero). The normal test was green; the required repeated race command was the RED evidence:

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

## Unverified consumer evidence and decision

The following commands were launched after the upgrade:

```sh
GOWORK=off go test ./internal/application/service -count=1
GOWORK=off go test ./internal/agent/recoverytest ./internal/agent/recoverytest/provider -count=1 -timeout=2m
```

The command host returned before either supplied a capturable terminal exit status/log. They are unverified, not passing evidence. The repeated native race gate is green, but this missing direct-consumer result, together with the pre-existing unselected persistent Session/Memory backend, append-failure barrier, PostgreSQL, real Provider, recovery, and client gates, keeps P0 **NO-GO** for native Runner product execution.

## Remaining work

Rerun the two unverified consumer commands in an execution environment that preserves terminal exit output, then complete the separate P0 persistence/recovery/provider/PostgreSQL/client acceptance gates. This Task 1 does not authorize product cutover.
