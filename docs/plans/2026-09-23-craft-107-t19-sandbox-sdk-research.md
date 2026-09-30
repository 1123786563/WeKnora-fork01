# T19 Task 4b — sandbox SDK/provider research

Research date: 2026-09-23. Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`. This note is read-only research; no production code or remote issue was changed.

## Verified facts

Pinned dependencies in `go.mod` are Docker Engine `github.com/docker/docker v28.5.2+incompatible`, Cube `github.com/tencentcloud/CubeSandbox/sdk/go v0.0.0-20260807115140-5cefcca27a7f`, and E2B `github.com/matiasinsaurralde/go-e2b v0.1.1-0.20260808041540-fdc08ceaa1c1`. Current application `remote_client.go` exposes only synchronous `Create`/`Exec`, matching the Task4a blocker report.

* Docker has an authoritative local exec ID and a genuine split: `Client.ContainerExecCreate(ctx, containerID, ExecOptions) (ExecCreateResponse, error)` returns an ID; `ContainerExecStart(ctx, execID, ExecStartOptions) error` starts it; `ContainerExecInspect(ctx, execID) (container.ExecInspect, error)` observes running/exit state. The SDK also exposes `ContainerCreate` returning a container ID and `ContainerInspect`. Sources: `$GOMODCACHE/github.com/docker/docker@v28.5.2+incompatible/client/container_exec.go:13-80`, `client/container_create.go:18-20`, `client/client_interfaces.go:74-83`.
* Docker's exec ID is provider-generated and `ExecStart` accepts no application idempotency key. Reusing the same ID is an observation/retry of the same exec, but there is no create-by-key or lookup-by-operation-key API in this pinned client. Therefore Docker can support bounded `StartExec` only if the adapter durably records the returned exec ID before/while starting and treats a lost response as unknown; it cannot prove “not started” from timeout alone, and it cannot provide charge-safe replay prevention from an operation key alone.
* Cube's public SDK `Client.Create(ctx, CreateOptions) (*Sandbox,error)` and `Client.Connect(ctx,sandboxID)` are synchronous. `CreateOptions` includes `Metadata`, `TemplateID`, timeout, environment and network fields; `SandboxInfo` includes `SandboxID`, metadata and state. Sources: `$GOMODCACHE/github.com/tencentcloud/!cube!sandbox/sdk/go@v0.0.0-20260807115140-5cefcca27a7f/client.go:66-100`, `models.go:9-36,56-80`.
* Cube's public `Commands.Run` is synchronous and returns only final stdout/stderr/exit code. Internally it calls an unexported `startProcess`; the response stream yields a PID and then drains completion, but the public API exposes neither a start handle nor reconnect/observe API. Sources: `commands.go:11-46`, `envd.go:26-120,246-290`. The PID is not a durable provider operation API and is unavailable from `Commands.Run`.
* E2B's pinned SDK has a useful command split: `CommandService.Start(ctx, cmd, ...RunOption) (*CommandHandle,error)` returns after the server start event and exposes `CommandHandle.PID() uint32`; `CommandHandle.Wait(ctx)` drains output and returns the final result. `CommandService.Connect(ctx,pid,...)` can reattach to an already-running process; `List` reports running processes. Sources: `$GOMODCACHE/github.com/matiasinsaurralde/go-e2b@v0.1.1-0.20260808041540-fdc08ceaa1c1/commands.go:93-177,185-233`, `command_handle.go:103-145`.
* E2B sandbox creation is synchronous: `Client.NewSandbox(ctx,cfg...) (*Sandbox,error)`, while `Connect(ctx,sandboxID,timeout)` and `ListSandboxes`/`ListSandboxesV2` provide recovery/lookup. `SandboxConfig.Metadata` is persisted and list-v2 supports metadata filtering. Sources: `client.go:69-151,302-405`, `sandbox.go:14-82`.

## Inference and limits

E2B is the strongest candidate for chargeable `StartExec`/`WaitExec`/`ObserveExec`: PID plus `Start`/`Connect`/`List` gives a provider operation identity and recovery path. However PID uniqueness/lifetime and server-side idempotency by application key are not established by the SDK; the adapter must persist `(sandbox ID, PID, operation key)` and classify start response loss as unknown. E2B `Start` itself opens a server stream and waits for the start event, so the callback must end immediately after that event; `Wait` must be outside the charge callback.

Docker can support the same shape for execution with the returned exec ID, but only as “start once, observe by ID”; add an adapter-level durable receipt before any retry. Docker create has a container ID but no idempotency-key or metadata lookup contract in this pinned client, so chargeable `StartCreate` should fail closed unless the surrounding adapter owns a deterministic name/label protocol and proves conflict/recovery semantics.

Cube must fail closed for chargeable execution initiation and create under the current SDK. Its public command API completes the operation, and its create API is synchronous; internal `startProcess` is unexported. Cube metadata/list support is useful for non-chargeable lifecycle reconciliation, but does not establish bounded create acceptance or operation-key idempotency.

## Recommended provider-neutral boundary

Use separate capabilities so unsupported providers cannot accidentally fall back to synchronous methods:

```go
type RemoteOperationRef struct { Provider, SandboxID, ID, OperationKey string }
type RemoteExecStarter interface {
    StartExec(ctx context.Context, h RemoteSandboxHandle, req RemoteExecRequest, key string) (RemoteOperationRef, error)
    WaitExec(ctx context.Context, h RemoteSandboxHandle, ref RemoteOperationRef) (*RemoteExecResult, error)
    ObserveExec(ctx context.Context, h RemoteSandboxHandle, ref RemoteOperationRef) (RemoteOperationObservation, error)
}
type RemoteCreateStarter interface {
    StartCreate(ctx context.Context, req RemoteCreateRequest, key string) (RemoteOperationRef, error)
    WaitCreate(ctx context.Context, ref RemoteOperationRef) (RemoteSandboxHandle, error)
    ObserveCreate(ctx context.Context, ref RemoteOperationRef) (RemoteCreateObservation, error)
}
```

The stable application key should be the persisted `CallID + attempt + operation kind`; provider IDs are receipts, not substitutes. Persist the provider receipt before waiting. If a start returns an error after the provider may have accepted, retain `Unknown` and recover by exact receipt/metadata lookup; never retry the chargeable operation. Only adapters with all three properties—bounded acceptance, authoritative receipt/lookup, and durable recovery—should advertise the capability. Initial support recommendation: E2B exec and Docker exec after adapter tests; Cube exec and all three providers' create paths remain fail-closed pending stronger provider contracts.

## Source pointers

* Task4a boundary and required recovery semantics: `docs/plans/2026-09-23-craft-107-t19-sandbox-initiation-task4a-report.md`.
* Application adapter files: `internal/modules/execution/sandbox/remote_client.go`, `docker_remote_client.go`, `cube_remote_client.go`, `e2b_remote_client.go`.
* Docker primary source: `github.com/docker/docker` v28.5.2 client methods cited above.
* Cube primary source: `github.com/tencentcloud/CubeSandbox/sdk/go` pinned module files cited above.
* E2B primary source: `github.com/matiasinsaurralde/go-e2b` pinned module files cited above.
