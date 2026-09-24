# RunView runtime R1 coordinator independent review

**Scope:** Exact checkpoint `internal/container/craft_runview_runtime.go` and `craft_runview_runtime_test.go`, reviewed read-only against the R1 plan/report, `docs/plans/runview-create-intent-review.md`, pinned OpenCode 1.18.4 inventory research, approved Craft Spec, RunView binding contract, and the current create-intent store interface.

## Verdict

- **Scoped Spec compliance: PASS.** `Resolve` obtains the persisted RunView generation through `Allocate`, derives the runtime/container identity and private directory from that generation, and does not accept a caller-supplied directory. It requires a verified dedicated container and complete, authoritative inventory for the exact directory and project. Only a unique session with matching observed directory/project can bind. A durable `BeginSessionCreate` winner alone may call `CreateSession`; unknown outcomes and absent/multiple/untrusted inventory return `ErrCraftRunViewRuntimeUnresolved` without a runnable handle or a second create. Bound rows are reloaded and checked against provider evidence.
- **Scoped code quality: PASS.** The coordinator's CAS use is consistent with the reviewed one-shot store contract. Errors in external creation are treated as uncertain, and retry/restart stays observe-only. The code does not dispatch prompts, wire a production provider, or claim process/OS isolation. The provider contract explicitly requires full cursor pagination and session-reported `location.directory`/`projectID`; it does not invent a generation field absent from pinned `Session.Info`.
- **Production RunView/T01 acceptance: NOT VERIFIED.** The checked-in OpenCode lock describes legacy `POST /session`, while the pinned source research identifies `/api/session` list/get routes. No live binary/provider proof, production assembly, or isolation evidence is in this checkpoint. A provider must prove dedicated generation-container identity and authoritative full inventory before this coordinator can be used to return runnable handles.

## Findings

No critical, high, medium, or low finding in the two-file R1 coordinator checkpoint.

## Evidence and limits

- SHA-256 matched the report: `craft_runview_runtime.go` `c4d1196bb8e943f5ca5d9f7b8874d6df2b7df81caf1ff3d8f6e8ddbe7d524a34`; `craft_runview_runtime_test.go` `50df0fc18c074a8122dc66ee377e5928876204ac81c983c90c752986a9c9a8de`.
- Independently ran `go test ./internal/container -run CraftRunViewRuntime -count=1`: passed. The linker emitted only `ignoring duplicate libraries: '-lc++'`. Focused fake-backed tests cover lost responses, restart, zero/partial/multiple inventory, wrong directory/project/container, bound revalidation, invalid root, and concurrent callers with one create.
- The provider interface's `Authoritative`, `Complete`, and identity flags are assertions supplied by a future provider. These tests establish the coordinator's fail-closed treatment of those assertions, not that a real provider can truthfully establish them from the pinned binary or that inventory is immediately consistent after create.
