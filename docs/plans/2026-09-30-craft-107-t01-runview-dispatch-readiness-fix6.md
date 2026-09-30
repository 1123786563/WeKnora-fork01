# CRAFT-107 T01 RunView dispatch readiness (research)

Date: 2026-09-30. Requested integration reference: `71809b47400e4201e33277dd5b46ae77b40d8625` (`feat(craft): 添加 DraftFencedVersionStore 接口实现预算扩展表及相关迁移`). This is a read-only investigation; no tests, runtime, database, Docker, or browser were run.

## Verified facts

* HTTP craft submission resolves the session owner and workspace, validates every `input_ref` against that workspace manifest, freezes the authorized inputs and model in a snapshot, and calls `AgentRunService.Submit` (`internal/application/service/craft_session.go`, `CraftSessionService.Submit`, around lines 700–800). Admission is therefore the durable Run identity boundary: `(tenant, owner, session, run_id)` is available only in the admitted Run/fence.
* The durable worker scans, claims a Run, optionally reconciles, then calls its injected `execute(ctx, fence)` (`internal/application/service/agent_run_worker.go`, `Tick`/`runOne`). The worker does not construct a Craft runtime or pass a workspace/input selection to an executor. Provider mode instead invokes the remote dispatcher and deliberately uses an executor that returns `trpc graph executor is not wired`.
* The actual graph path reconstructs capabilities from the admitted snapshot, restores history/checkpoint state, obtains the run-scoped journal/input/event stores, builds the graph, and executes it (`internal/application/service/agent_run_graph.go`, durable executor closure and `GraphBindings`). This is the path that must ultimately call a Craft task executor.
* Current production Craft assembly still constructs `localCraftRuntime` from `CRAFT_OPENCODE_BASE_URL`, a shared `workDir`, shared `sessionsRoot`, and one `opencode.Client`; `localCraftRuntime.Execute` calls `ensureWorkspace`, stages inputs, points output, then invokes `opencode.Executor` (`internal/container/craft_runtime.go`). `ensureWorkspace`/OpenCode session state is task/workspace based, so it is not a RunView-bound runtime.
* The T01 implementation exists in the earlier ancestor `8a3e2d211`, where `CraftRunViewRuntimeCoordinator` and `assembleCraftRunViewProduction` were introduced. Its own comment says the coordinator “is not wired into production.” That assembly created `ResolveMaterial`, but current live `newCraftRuntimeExecutor` does not accept or use `CraftRunViewRuntimeCoordinator`; it returns the legacy `localCraftRuntime` directly. The current `container.go` has no `provideCraftRunViewProductionAssembly`/`ResolveMaterial` hookup. This is the precise gap: a coordinator can be constructed (in the T01 assembly seam) yet no admitted Run dispatch invokes it before OpenCode session creation, input staging, knowledge writing, or executor calls.
* Existing T01/T20 evidence explicitly records this state: `docs/plans/2026-09-23-craft-107-runview-runtime-adapter-plan.md` says the adapter is a usable interface, not a deployed view; `docs/plans/2026-09-23-craft-107-t20-report.md` records local serve as “RunView production pins are incomplete” and capture as inert. These are deployment/assembly facts, not proof of Run isolation.

## Inference

The smallest safe slice is an assembly-and-dispatch seam, not a new allocator: make the Craft executor receive a server-owned admitted-Run resolver and require `ResolveAdmitted(task)` before any side effect. The resolved handle must supply the directory-bound client/session and private root to Execute/Observe/Abort, while material staging and artifact collection consume that same handle. Keeping only a constructed coordinator or only calling it for artifact capture leaves the legacy shared `workDir` and OpenCode session path active.

The first proving slice should cover two sequential Runs on one Task: Run A with no selected input and Run B with a selected input (or the inverse depending on the fixture). Each execution must record distinct RunView generation/runtime/session/root identities; the executor must see only the admitted Run's input manifest. A cross-Run fake provider/session event must be rejected before it reaches the Craft event stream. This isolates the dispatch seam without requiring end-to-end provider behavior.

## Recommended implementation ownership and interfaces

1. `internal/container/container.go`: central assembly owner. Reintroduce/retain the RunView production assembly as a dependency of the Craft runtime, and fail closed when pins/provider/coordinator are unavailable. Do not make `localCraftRuntime` silently fall back to shared `workDir` for Craft.
2. `internal/container/craft_runtime.go`: runtime owner. Add a narrow `RunRuntimeBinding`/resolver dependency to `localCraftRuntime`; on Execute/Observe/Abort resolve from the admitted `craft.Task.Fence` and `craft.Task.Scope`, then use the returned directory-bound client/root for all staging and OpenCode calls. Remove the shared workspace/session path from the production execution branch once the resolver is required.
3. `internal/modules/agentruntime/agent/opencode/executor.go` and `client.go`: OpenCode owner. Accept the resolved per-Run client/session through a per-call binding port; no fallback to `craft_workspaces.oc_session_id`. Event normalization must match Run session and message IDs.
4. Tests: focused container/runtime tests beside `craft_runtime.go` plus executor binding tests. Required assertions: (a) zero-input Run resolves and executes in its own generation; (b) selected-input Run resolves a different generation/root and receives only its selected manifest; (c) second Run cannot reuse first session/root or foreign event; (d) missing resolver/pins fails closed before provider/session creation.

## Dependencies and blockers

* The worker/graph seam must expose an admitted Craft `Task`/fence to the Craft executor; a generic `agent_run_worker` callback alone has no input manifest or Craft workspace. The graph snapshot must remain the source of selected input authorization.
* Production pins (`CRAFT_RUNVIEW_*`) and an isolated container/provider must be enabled before live evidence can claim isolation. Existing T20 evidence says they are incomplete in the default local serve deployment.
* T05 material publisher/knowledge writer must consume the same RunView handle; otherwise selected inputs can be correctly resolved but knowledge or output still lands in the shared runtime.
* This report does not certify the current dirty worktree as equivalent to requested HEAD; inspected live source hashes are below.

## Inspected source hashes (SHA-256)

| Source | SHA-256 |
|---|---|
| `internal/application/service/craft_session.go` | `418725eeeeeaf98e11007ed8519db99078384b8ff8e6bdb4c32375b3deb5f465` |
| `internal/application/service/agent_run_worker.go` | `1100f7c06361ff1bc414ccb434be192da2f13409f9ecf4f1c7eb406efc1be98c` |
| `internal/application/service/agent_run_graph.go` | `3197d7d39553be193a3267153fea18e88547cfff6255acfb1cda4c21d1d1fa8f` |
| `internal/container/container.go` | `143ffa72f82b0b259becb1f1ddacca2b82b414729675a3d49c82fa9a5b3c8556` |
| `internal/container/craft_runtime.go` | `47b69db234d8e2ad1af12f75f7f35f732a91ab59cf2aefd4b15f04417622aa6e` |
| `internal/modules/agentruntime/agent/opencode/executor.go` | `d735a7fa1170de31eafd7e7d21b55ce57982d0c1633b3c242ca81af39dc75a34` |
| `internal/modules/agentruntime/agent/opencode/client.go` | `28e3df68faf12a29d65ad77c4dd9ea43a72eb4691f11e7ba94d231c2d36db881` |
