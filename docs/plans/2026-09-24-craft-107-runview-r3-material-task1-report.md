# T01 RunView R3 Task 1 report — verified generation material handle

**Status:** DONE_WITH_CONCERNS. No commit created. The Task 1 boundary is implemented and focused tests pass. The complete `internal/container` package remains red in two assembly tests unrelated to this task.

## Change

Added `CraftRunViewContainerProvider.MaterialHandle`, which accepts the server's admitted Run key, persisted runtime handle, and RunView store. Before returning an opaque handle with unexported host paths, it validates the RunView, reloads and compares the persisted record, checks generation-derived runtime/container/directory identities, revalidates the provider's current complete engine/network/mount/runtime-probe evidence, and GETs the exact bound OpenCode session to verify ID, project and directory. It derives the generation root from configured `SandboxRoot`; it does not accept a caller path. Existing secure generation layout checks reject symlinked root children and canonical-path escape.

The focused tests cover valid generation layouts, separate roots for two Runs on one Task, cross-Run handle use, forged generation/container/session/directory, unbound and stale persisted bindings, wrong persisted scope, missing container, inspected mount/network/image/probe changes, and a symlinked child directory. These fake-engine tests do not establish production Linux mount isolation.

## RED → GREEN and verification

- RED: `go test ./internal/container -run '^TestCraftRunViewMaterialHandle' -count=1` failed to compile because `MaterialHandle` did not exist.
- GREEN: `go test ./internal/container -run '^TestCraftRunViewMaterialHandle' -count=1` — PASS.
- Focused provider + material suite: `go test ./internal/container -run '^TestCraftRunView(ContainerProvider|MaterialHandle)' -count=1` — PASS.
- Race check: `go test -race ./internal/container -run '^TestCraftRunView(ContainerProvider|MaterialHandle)' -count=1 -timeout=120s` — PASS.
- Formatting: `gofmt -d internal/container/craft_runview_material.go internal/container/craft_runview_container_provider_test.go` — PASS (no output).
- Whitespace: `git diff --check -- internal/container/craft_runview_container_provider.go internal/container/craft_runview_container_provider_test.go internal/container/craft_runview_material.go` — PASS; `git diff --no-index --check /dev/null` against the two new/modified untracked files also produced no diagnostics.
- Full container package: `go test ./internal/container -count=1` — FAIL in `TestCraftAccessFeatureRegistriesAreAssemblyOwned` (missing `craft.TaskAccessChecker` dependency) and `TestWireCraftInteractionRegistrarRegistersPendingInteractions` (`agent runtime conflict`). The focused provider/material tests passed; these package-level failures are outside Task 1's owned files.

## Exact checkpoint

Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`.
Starting/checkpoint HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit created.

Task-start hashes were recorded before editing. The provider source itself is unchanged. Its baseline was reconstructed from `2026-09-23-craft-107-runview-provider-fix1-checkpoint.patch`; reconstructed SHA-256 values match the task-start hashes. The exact task delta is in `2026-09-24-craft-107-runview-r3-material-task1-task-local.patch`; checkpoint hashes are in the adjacent JSON manifest.

- `internal/container/craft_runview_container_provider.go` — pre `2872bb3360bd4f85c52185ae8ea24419089a4b8f4369fedac7ff88fa1d98ae6d`; post same.
- `internal/container/craft_runview_container_provider_test.go` — pre `edc72c3dda199405148a198885cbfc6cad091559c2b086f69b5d4e15d12497ae`; post `519da4d8c7a3f1eef02ed92cfb40a5c1c41fce0e0f9a669e851f52c9d8c59ebe`.
- `internal/container/craft_runview_material.go` — new; post `676902fc87f2267249c0abc909e3019df1c9d5ee86ef2cd44148e7d514e6a8b9`.

## Scope and limits

No production runtime assembly, coordinator/store source, API DTO, prompt, or Linux isolation wiring was changed. `MaterialHandle` requires the provider's current in-memory verified binding; normal coordinator resolution must happen first, including after restart. Production engine, pinned Linux image, mount isolation, and Run A→B proof remain separate release gates.
