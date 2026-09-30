# T19 sandbox Task 4b — Task 2 provider feasibility report

**Status: BLOCKED / FAIL-CLOSED.** No provider adapter was changed or advertised because neither reviewed candidate satisfies durable same-key recovery within the assigned adapter seam. This report is the Task 2 checkpoint; source files remain unchanged. No commit/staging occurred.

## Scope and evidence

Reviewed Task 2 in `2026-09-23-craft-107-t19-sandbox-task4b-plan.md`, Task 1's independent PASS review and fix checkpoint, `2026-09-23-craft-107-t19-sandbox-sdk-research.md`, and the pinned Docker/E2B SDK source in the local Go module cache.

Current adapter inspection:

- `DockerRemoteClient.Exec` performs `ExecCreate`, `streamExec` (which attaches and drains output), then `ExecInspect`. The pinned Docker client offers separate exec-create, exec-start and inspect methods, so a bounded adapter start can be formed once the returned exec ID is known. `ExecStart` accepts only an exec ID and options; it does not accept the stable application operation key. There is no Docker API to list/find an exec by application key. A lost `ExecCreate` response leaves no ID to inspect; an adapter-local ID map would not survive process restart. Existing Docker create generates a daemon ID, starts synchronously, and has no deterministic create-by-key or durable receipt boundary in this task's files.
- Pinned E2B `CommandService.Start` is bounded through the start event and returns a PID. `Connect(pid)` and `List()` can observe/re-attach to currently running processes. The SDK does not accept the application operation key in `Start`; its reported process tag is not exposed as a start option. There is no durable mapping from the stable operation key to PID. `List()` reports currently running processes, so it cannot recover a command that already finished during a response-loss/restart window. An in-memory mapping inside `E2BRemoteClient` would not satisfy restart recovery.
- Neither adapter nor the optional provider-neutral contract currently owns a persistent receipt store. Adding one requires a reviewed durable storage seam beyond the assigned adapter-only files.
- No current Docker/E2B adapter source implements `StartExec`, `WaitExec`, `ObserveExec`, `StartCreate`, `WaitCreate` or `ObserveCreate`; the scoped `rg` returned no matches. Existing synchronous `Create`/`Exec` behavior remains intact.

SDK source locations observed:

- Docker v28.5.2: `$GOMODCACHE/github.com/docker/docker@v28.5.2+incompatible/client/container_exec.go` (`ContainerExecCreate`, `ContainerExecStart`, `ContainerExecInspect`); application use in `docker_remote_client.go` `Exec` and `Create`.
- E2B pinned go-e2b: `$GOMODCACHE/github.com/matiasinsaurralde/go-e2b@v0.1.1-0.20260808041540-fdc08ceaa1c1/commands.go` (`Start`, `Connect`, `List`); `command_handle.go` (`PID`); application use in `e2b_remote_client.go` `Exec` and `Create`.

## Decision

Do not implement or advertise any provider capability in this task. Docker exec ID and E2B PID are provider receipts after the SDK returns them, but neither lets this adapter recover a lost response by the stable operation key after restart. Treating a later absent lookup as proof of “not started” would permit duplicate physical work. All current chargeable provider initiation remains unsupported at the optional capability boundary. Provider create remains unsupported as well.

To proceed, a separately reviewed durable receipt or authoritative provider key-lookup seam is needed. It must record/recover `(provider, sandbox, operation key, provider receipt)` before completion wait and prevent a second physical start after an ambiguous outcome. Task 2 is stopped pending that prerequisite, as directed by the brief.

## Verification

```text
rg -n 'StartExec|WaitExec|ObserveExec|StartCreate|WaitCreate|ObserveCreate' internal/modules/execution/sandbox/docker_remote_client.go internal/modules/execution/sandbox/e2b_remote_client.go
No matches (exit 1)

go test ./internal/modules/execution/sandbox -run '^TestRemoteOperation' -count=1
ok github.com/Tencent/WeKnora/internal/modules/execution/sandbox 1.034s

go test ./internal/modules/execution/sandbox -count=1
ok github.com/Tencent/WeKnora/internal/modules/execution/sandbox 5.615s
```

These package tests are regression evidence for the unchanged fail-closed contract and existing adapters. They do not claim live provider, response-loss, cross-process recovery or charge integration evidence.

## Exact source checkpoint

The pre/post source hashes are identical because no source/test file was changed in this Task 2 feasibility checkpoint:

```text
internal/modules/execution/sandbox/docker_remote_client.go  dfb98bc2695b863f6630c10a484c66197fa40ee44aa5912e29918e3da52e36a8
internal/modules/execution/sandbox/docker_remote_client_test.go  e98d3be9085889e1f8c73a31d897159e53cf8104a5714f3575597b96380a9c3d
internal/modules/execution/sandbox/e2b_remote_client.go  84effaf3f01545136e8a4d814941383f863d5ccac45461a89461d6a0b62c92a2
internal/modules/execution/sandbox/e2b_remote_client_test.go  efe7f75c23ecdcb3d5fcb4cf9c6ed126fb7a73486f39dfba5c870e17193add8c
internal/modules/execution/sandbox/remote_operation.go  a6c678529485e9eb5a11ff16ad71c5a9b17ee3f25b48411386f8b90ddb7d99a3
internal/modules/execution/sandbox/remote_operation_test.go  7fcb99d0aa050f19d4602daabb0d77da43e49f241f5cec9914cf04097fee5462
```

No task source patch exists for this checkpoint because no source change was safe. This report is the evidence artifact; do not proceed to provider wiring until the durable receipt/recovery prerequisite is reviewed.
