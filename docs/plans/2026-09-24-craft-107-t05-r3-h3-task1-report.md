# Craft #107 T05/R3 H3 Task 1 — DI gap report

## Outcome

**Gated; no production or test files changed.** Parent direction was to avoid wiring H1 through an absent production RunView provider or introducing a dummy/default-on path. H3 Task 1 must resume after T01 R5 supplies a reviewed production provider/resolver assembly.

## Evidence collected

- `internal/container/container.go:495-500` registers `newCraftRuntimeExecutor`, whose documented default is the unavailable executor unless `CRAFT_OPENCODE_BASE_URL` is set.
- `internal/container/container.go:523-527` registers `newCraftKnowledgeService`.
- `internal/container/container.go:2615-2656` shows that `newCraftKnowledgeService` returns nil only when the executor is not `*localCraftRuntime`; otherwise it writes knowledge files beneath the shared `runtime.workDir` using `MkdirAll` and `WriteFile`. This is the legacy shared writer H3 is required to remove.
- `internal/container/craft_runtime.go:91-96` shows that `newCraftRuntimeExecutor` accepts DB, Craft store, AgentRunStore, files, versions and previews, but no RunView store/provider or knowledge builder.
- `internal/container/craft_runtime.go:168-184` shows the local runtime's material, resolver and verifier hooks. The constructor found above does not populate them; `Execute` fails closed when they are nil.
- `internal/container/craft_runview_container_provider.go:218-257` shows provider construction requires an engine, exact pinned image/binary/runtime digests, pinned OpenCode version, project ID and dedicated sandbox root. No corresponding production provider construction/registration exists in `container.go`.
- `internal/container/craft_knowledge_runview.go:18-56` shows H1's builder requires Craft store, knowledge access/search, TaskAccessChecker and durable `CraftKnowledgeRecordStore`; the builder consumes a provider-issued material handle and durable `agentruntime.Run`.
- The durable Run store is available (`repository.NewAgentRunStore`, `AgentRunStore.Get`), Task ACL is registered (`service.NewCraftAccessService` plus `craftTaskAccessChecker`), and knowledge access/search adapters are already created by the old constructor. `repository.NewCraftKnowledgeRecordRepository` exists but is not registered in `container.go`.
- The H1 and H2 reports confirm package building and final-byte verification are implemented and reviewed independently; they do not provide production provider assembly.

## Minimum prerequisite interface for T01 R5

R5 must provide a production-constructible, pinned `CraftRunViewContainerProvider` and its durable `craft.RunViewStore`, then expose a server-owned material resolver that:

1. derives `RunKey` from the admitted Craft task/fence, reloads it via `AgentRunStore.Get`, and validates tenant, Task/session identity, actor, generation and immutable snapshot;
2. obtains the provider-issued `CraftRunViewMaterialHandle` for that exact persisted RunView;
3. binds the H1 builder callbacks per Run, using the same handle and accepted package for `BuildForRun` and H2 `VerifyAcceptedForRun`;
4. fails closed if any store, provider, actor, snapshot or ACL/record dependency is missing.

Once that interface is reviewed, H3 can wire the durable record repository and real ACL ports, install the resolver/verifier on the local runtime at startup, and delete the shared `workDir` writer. No workspace-root fallback is valid.

## Commands and results

- `rg -n "Provide\\(.*(CraftRunView|RunView|KnowledgeRecord)|NewCraftRunViewContainerProvider|newCraftKnowledgeService|newCraftRuntimeExecutor|func NewCraftKnowledgeRunViewBuilder" internal/container/container.go internal/container/craft_runtime.go internal/container/craft_knowledge_runview.go` — source inspection confirms `newCraftRuntimeExecutor` and `newCraftKnowledgeService` registrations; no production `CraftRunViewContainerProvider` construction/registration or knowledge record repository registration in `container.go`.
- `sed -n '88,155p' internal/container/craft_runtime.go` and `sed -n '165,240p' internal/container/craft_runtime.go` — confirmed constructor inputs omit RunView provider/builder and runtime hooks fail closed when unset.
- `sed -n '218,257p' internal/container/craft_runview_container_provider.go` — confirmed provider requires pinned image/runtime integrity inputs and an engine.
- `sed -n '18,58p' internal/container/craft_knowledge_runview.go` — confirmed H1 dependency contract.
- Tests were not run: this is a read-only assembly gap ruling, and no production implementation was authorized after the gap was found.
- No commit created. Shared worktree changes owned by other tasks were left untouched.

## Remaining risk / next action

H3 Task 1 acceptance remains unmet: the releasable path still has legacy shared-writer assembly code, while the current runtime hooks are unbound and therefore fail closed. Resume only after T01 R5's provider/resolver contract is present and reviewed; keep the production dial disabled until the full downstream gates pass.
