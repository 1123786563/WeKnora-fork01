# T05 Production Worker Seam Map

Read-only source trace captured at HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34` in the integration Worktree. This map identifies the production seams for T05 knowledge assembly and dispatch-time authorization. No production or test source was changed, and no shared tests were run. T05 is not verified by this map.

## Current admission and worker path

1. `internal/handler/session/craft.go` decodes `craftRunRequestDTO.KnowledgeScope` (`knowledge_scope`) and passes it to `service.CraftRunRequest.KnowledgeScope`.
2. `internal/application/service/craft_session.go:CraftSessionService.StartRun` (`~766`) validates Task write through `writeSession`, resolves upload refs, then calls `craftRunSnapshot` (`~953`) and persists it using the durable run submission boundary. The snapshot query includes the prompt, upload descriptors and base version. **It does not freeze `KnowledgeScope` or any knowledge selection.** `KnowledgeScope` currently behaves as request metadata only.
3. `internal/application/service/agent_run_graph.go:DurableRunSnapshot` contains `Query`, runtime/model identity and `CraftInputManifest`; `BuildDurableCraftRunSnapshot` freezes only the authorized upload manifest. The durable row key `agentruntime.RunKey` carries the authoritative tenant and `RunID`.
4. The production graph is registered by `internal/application/service/session.go` (`RegisterGraphExecutor(svc.ExecuteDurableRun)`). `sessionService.ExecuteDurableRun` (`internal/application/service/agent_run_graph.go:348`) loads the fenced row, verifies owner/epoch, injects tenant/principal context, parses the immutable snapshot, restores config, and calls `agentService.prepareAgentCapabilities` (`internal/application/service/agent_capabilities.go:84`). This is the correct source for any server-only selection: use the admitted snapshot and `fence.RunKey.RunID`, never model tool arguments or current workspace rows.
5. Capability assembly reaches `agentService.registerCraftDelegateTool` (`internal/application/service/agent_service.go:~1137`, implementation in `internal/application/service/craft_delegate.go:306`). It currently derives scope/workspace from persisted session state and registers the tool with scope, workspace, skill digests and delegate callback. It receives neither RunID nor the admitted knowledge selection/query.
6. `internal/modules/agentruntime/agent/tools/craft_delegate.go:CraftDelegateTool.Execute` obtains `RunFenceFromContext`, durable tool-call identity and authorized upload refs, then constructs `craft.Task` and invokes the delegate. The fence supplies the RunID; the task has no T05 selection payload.
7. The final application dispatch boundary is `CraftDelegateService.Delegate` (`internal/application/service/craft_delegate.go:103`). After replay/unknown handling and lifecycle `GuardDispatch`, it prepares the durable delegation and calls `s.executor.Execute(ctx, prepared)`. There is currently no TaskWrite or source ACL recheck on this path.
8. The real OpenCode executor is `localCraftRuntime.Execute` (`internal/container/craft_runtime.go:180+`). It independently resolves uploads through `admittedRunInputManifest`, loading the durable snapshot by `(tenant_id, run_id, session_id, owner_id)`. That is an existing example of a trusted RunID-to-snapshot lookup. Knowledge has no equivalent worker binding yet.

## T05 service and wiring status

- `CraftKnowledgeService.BuildForRun` (`internal/application/service/craft_knowledge.go:160`) requires `TaskWrite`, builds or resumes one immutable per-Run record, reauthorizes the source document/KB grants, and publishes a complete package. Its current `selectedKnowledgeIDs` are **document IDs**; it requires at least one and resolves them via the shared-aware document access port before ACL-guarded retrieval.
- Approved Spec user story 2 says members select specific **knowledge bases**. The current client submits `knowledge_scope` as a single KB ID (`packages/views/src/craft/home.tsx`, `apps/web/src/features/craft/routes.tsx`, `packages/api-client/src/craft/index.ts`). A KB ID must not be passed as if it were a document ID. Admission/service selection semantics need an explicit typed contract aligned with the Spec (including whether one or multiple KBs are selectable).
- `newCraftKnowledgeService` (`internal/container/container.go:2623`) currently injects Store/Access/Search/Writer only, and binds its writer to `localCraftRuntime.workDir`. It does not inject TaskAccess, durable Records or the atomic Publisher required by `BuildForRun`; this provider is not a production-ready T05 assembly. `container.go` is owned by the concurrent T08 wiring work and is intentionally untouched here.
- `NewCraftKnowledgePackagePublisher` (`internal/container/craft_knowledge_publisher.go:88`) binds the publisher to a canonical server-owned root and exact RunID. The adapter and focused tests exist, but their presence does not provide per-Run OS/runtime read isolation. Do not bind its root to shared `runtime.workDir`.
- `internal/modules/craft/run_view.go:RunViewKey`, `RunViewRuntime`, and `RunViewStore` currently persist exact identity and runtime IDs only. `RunViewRuntime` intentionally has no path or mount authority. This is not enough to establish a selected-material-only filesystem boundary.

## Proposed stable handoff interfaces

First settle the selection type against the approved KB-selection behavior. The currently compatible direction is a server-owned typed value in the durable snapshot, for example:

```go
type CraftKnowledgeSelectionSnapshot struct {
    Query            string   `json:"query"` // exact admitted user retrieval query, independent of model-facing prompt text
    KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
}

type DurableRunSnapshot struct {
    // existing fields ...
    CraftKnowledge *CraftKnowledgeSelectionSnapshot `json:"craft_knowledge,omitempty"`
}
```

`CraftSessionService.StartRun` must validate/canonicalize that selection before snapshot serialization; snapshot bytes already feed the idempotency digest. `Query` should be the original admitted prompt unless the product explicitly chooses a separate retrieval query. `RunID` remains the durable row/fence identity and should not be duplicated from client input.

The current `BuildForRun(ctx, scope, runID, query, selectedKnowledgeIDs)` contract cannot consume `KnowledgeBaseIDs` safely. Before wiring, either (a) evolve the service to a typed KB selection and perform authorized retrieval only inside those KBs, verifying returned document ACLs, or (b) revise admission/UI/spec so it submits explicit authorized document IDs. Do not silently reinterpret either identifier. A typed service seam should look like:

```go
BuildForRun(ctx context.Context, scope craft.Scope, runID string,
    selection CraftKnowledgeSelectionSnapshot) (craft.KnowledgeBundle, error)
RevalidateForDispatch(ctx context.Context, scope craft.Scope, runID string) error
```

The second method is a proposed narrow operation: load the accepted record using exact scope+RunID, require published state, require current `TaskWrite`, and reauthorize every recorded document and owning KB. It should not rebuild or republish during delegation dispatch. The existing `BuildForRun` replay does recheck authority but also resumes/publishes; calling that at every dispatch obscures the authorization seam and may do I/O. Keep the snapshot selection inaccessible to model-controlled `craft_delegate` arguments.

At graph assembly, pass the parsed snapshot selection and authoritative `fence.RunKey.RunID` through a server-only capability input to Craft delegate registration. At `CraftDelegateService.Delegate`, derive scope from the validated task/fence and invoke `RevalidateForDispatch` immediately before `PrepareTask`/executor dispatch. For retries/unknown outcomes, preserve existing observe-only behavior; do not authorize a second execution until the durable state proves it is a fresh dispatch.

## Ordered implementation tasks and ownership

1. **Selection contract/admission (T05 service/API owner):** reconcile KB selection with current document-ID service input; add typed snapshot selection, validation, canonical hashing and admission tests. Preserve the admitted query and IDs exactly. No UI identifier may be promoted into authorization without an ACL lookup.
2. **Run-scoped capability assembly (worker/graph owner):** pass parsed snapshot selection and `fence.RunKey.RunID` through `ExecuteDurableRun` → `prepareAgentCapabilities` → `registerCraftDelegateTool` using a server-only config field. Add a focused test that model-supplied tool arguments cannot alter selection.
3. **Dispatch recheck (Craft delegate/service owner):** inject a narrow revalidation port into `CraftDelegateService`; before a genuinely new executor call, verify current TaskWrite and recorded source grants for this exact scope+RunID. Add revoked TaskWrite/document/KB tests at the final dispatch seam; keep stored-result and unknown-outcome behavior safe.
4. **Production assembly (container owner, currently T08):** provide durable record store, TaskAccess and per-Run publisher for `CraftKnowledgeService`; do not target shared `localCraftRuntime.workDir`. Validate missing dependencies fail closed.
5. **RunView runtime boundary (runtime/RunView owner):** extend the runtime contract so a RunView binds an enforceable isolated root/mount identity, not merely IDs. Create fresh Run-specific execution filesystem; publish selected files read-only into that view, with no path from Run B to Run A. Only then bind `NewCraftKnowledgePackagePublisher` to that root and OpenCode session.
6. **Integrated proof (central integration owner):** exercise admission → durable worker → T05 package → real delegate dispatch in Run A/B, replay/recovery, TaskWrite revocation, document/KB revocation, and sandbox read attempts against other Run roots. Retain evidence from the production publisher and runtime boundary; unit tests of the service/publisher are insufficient for T05 completion.

## Unresolved production gate

The current RunView record has no enforced filesystem root, and the current local Craft runtime uses shared persistent serve `workDir` plus per-delegation workspace behavior. A database identity, directory naming convention, Unix mode, or publisher-root path alone does not prove that OpenCode for Run B cannot read Run A material. The runtime must establish a fresh per-Run sandbox/session and a read-only selected-material boundary enforced by mount/copy plus process isolation. This remains a central runtime design/integration prerequisite; no T05 verification claim is made here.
