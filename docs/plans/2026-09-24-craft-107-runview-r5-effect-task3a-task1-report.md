# R5 Effect Task 3a — Task 1 report

**Checkpoint:** `craft107-r5-effect-task3a-task1`  
**Plan:** [R5 Effect Task 3a Admitted Coordinator Plan](2026-09-24-craft-107-runview-r5-effect-task3a-plan.md)  
**Worktree:** `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`  
**Base HEAD:** recorded in `2026-09-24-craft-107-runview-r5-effect-task3a-task1-checkpoint.json`.  
**Commit:** none.

## Implementation

Added `CraftRunViewRuntimeCoordinator.ResolveAdmitted(ctx, originalTask)` and an admitted coordinator constructor. It passes the exact Task, including its Run fence and snapshot digest, unchanged to `AllocateAdmitted` and every `BeginEffect`; it rejects inconsistent Task/Fence identity before allocation. The existing key-only `Resolve` remains unchanged for compatibility. The admitted path does not call key-only `Allocate`.

Every provider mutation in the new path requires an authority claim whose key, generation, kind and opaque token match the original Task and current RunView generation. Docker create, Docker start and OpenCode create are separate typed effect stages. Successful finishes carry an observed container ID, running-container receipt, or unique scoped session ID. Errors/cancellation after claim are recorded as unknown when possible with a bounded cancellation-independent finish context. Replays observe only; they do not resend an operation whose claim is already used. The legacy RunView store's `BeginSessionCreate` is retained only as the existing durable binding marker needed by `BindRuntime`; its `maySend` result is ignored, and the effect claim is the sole send authority.

The new `CraftRunViewAdmittedEffectProvider` interface is deliberately narrow:

- Read-only: `ObserveContainer`, `ObserveContainerState`, `ObserveSessions`.
- One-stage mutations: `CreateGenerationContainer`, `StartGenerationContainer`, `CreateOpenCodeSession`.
- Observations must not create networks, containers, sessions or filesystem state. Each mutation method performs only its named stage. Generation network provisioning must already be available or the stage fails closed because no durable network-create effect kind exists.

The coordinator checks this interface before making any provider call. The existing `CraftRunViewContainerProvider` only implements the legacy combined interface, so it is rejected fail-closed. No provider/engine, central DI, production runtime dial, feature enablement, T19, R4, or application-repository file was edited.

## RED → GREEN evidence

RED command:

```text
go test ./internal/container -run '^TestCraftRunViewAdmittedCoordinator' -count=1
```

Before the implementation, it failed to compile because `NewCraftRunViewAdmittedRuntimeCoordinator` and `ResolveAdmitted` did not exist. This was the expected missing-seam failure.

Final postimage verification:

```text
go test ./internal/container -run '^TestCraftRunViewAdmitted' -count=1
go test -race ./internal/container -run '^TestCraftRunViewAdmitted' -count=1
go test ./internal/container -run '^$' -count=1
git diff --check
```

All passed. The focused tests use a fake split provider and authority to verify claim-first barriers, transition-first denial, current Task/fence/digest use, stale Run rejection by authority, rejection of the legacy combined provider, all three effect receipts, unknown/cancellation no-resend, changed container identity, stable generation replay, and zero provider calls when claims are denied. The Go linker printed the existing duplicate `-lc++` warning; tests returned exit code 0. The T19 owner was notified that the shared package test window was released after all commands finished.

## Exact interface blocker and Task 3b boundary

The current real provider cannot implement this protocol without an owned Task 3b refactor. `CraftRunViewContainerProvider.InspectOrCreateContainer` currently runs `EnsurePrivateNetwork`, inspection, possible filesystem layout/marker creation, Docker create, then start/probe in one call. In addition, `FindSessions` and `CreateSession` use `currentBinding`/`verifyCurrent`, which calls `EnsurePrivateNetwork` again. The Docker engine seam does separate `CreateContainer` and `StartContainer`, but the provider hides them in the combined method. Therefore the admitted path must remain unwired/fail-closed until the adapter provides the split interface and read-only observations.

Task 3b provider/engine implementation ownership should explicitly include:

- `internal/container/craft_runview_container_provider.go` and `craft_runview_container_provider_test.go`: separate pure inspection from network/container mutations; implement exact generation identity observation, one-step create/start, read-only scoped session inventory, and one OpenCode create request with post-call identity observation.
- `internal/container/craft_runview_docker_engine.go` and its focused engine tests: preserve distinct Docker create/start observations and IDs; ensure absence/uncertainty and response loss remain distinguishable.
- `internal/container/container.go` and `craft_runview_assembly_test.go`: assemble only the admitted coordinator/provider path behind the existing default-off configuration; keep the legacy key-only coordinator out of production Craft assembly.

There is an additional gate: generation-scoped `EnsurePrivateNetwork` can itself create a Docker network, but the R5 effect contract currently has no network-create kind. Task 3b must use a server-provisioned private network that is already available and only inspect/validate it, or stop for an authorized contract/storage extension. It must not hide network creation under Docker create/start. This Task adds no production provider capability and makes no claim that a physical provider operation is now reachable.

## Exact checkpoint

- Preimage archive: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task3a-task1-preimage.tar.gz`
- Preimage manifest: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task3a-task1-pre.sha256`
- Task-local patch: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task3a-task1-task-local.patch`
- Postimage archive: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task3a-task1-postimage.tar.gz`
- Postimage manifest: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task3a-task1-post.sha256`
- Machine-readable checkpoint: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task3a-task1-checkpoint.json`

The checkpoint patch contains only `craft_runview_runtime.go` and the new `craft_runview_admitted_runtime_test.go`; aggregate worktree changes from other streams are excluded.
