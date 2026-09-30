# Craft #107 T01 RunView R5 engine Task 2 report

Checkpoint: `t01-r5-engine-task2-assembly-1`  
Workspace: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`  
Base: `a5e9195acd6500c085c85d60c852148e7bbbbf34`  
Commit: none.

## Delivered

- Registered a lazy `CraftRunViewProductionAssembly` DI provider for downstream H3 wiring. Its unavailable state is nonfatal and default-off; it only constructs the Docker client when every required deployment pin is present.
- Read explicit server deployment pins from `CRAFT_RUNVIEW_SANDBOX_ROOT`, `CRAFT_RUNVIEW_IMAGE_REFERENCE`, `CRAFT_RUNVIEW_IMAGE_DIGEST`, `CRAFT_RUNVIEW_RUNTIME_CONFIG_SHA256`, `CRAFT_RUNVIEW_PROJECT_ID`, and `CRAFT_RUNVIEW_DOCKER_ENDPOINT`. The Linux OpenCode 1.18.4 binary digest comes from the checked-in lock. The constructor requires the reference to end in the separately configured digest; it does not infer one from a local image ID or daemon RepoDigest.
- Assembled the real Docker engine, `NewCraftRunViewContainerProvider`, `NewCraftRunViewStore(db)`, and `CraftRunViewRuntimeCoordinator`. `ResolveMaterial` derives its key from the server-owned Task scope/fence, runs coordinator recovery and current container/session verification, then issues a provider-bound material handle. Missing, canceled, foreign, unbound, or uncertain runtime state returns an error. No legacy `workDir` path is read or used as fallback.
- Exposed a `Close` callback for the owned Docker client. The integration remains default-off and this Task does not enable a production route, publish an image, or wire H3.

## Tests and evidence

- `go test ./internal/container -run '^TestCraftRunViewProductionAssembly' -count=1` — PASS.
- `go test ./internal/container -run '^TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity)' -count=1` — PASS.
- `git diff --check` — PASS.
- Coordinated `go test ./internal/container -count=1` — FAIL only in existing out-of-scope `TestCraftAccessFeatureRegistriesAreAssemblyOwned` (missing `craft.TaskAccessChecker`) and `TestWireCraftInteractionRegistrarRegistersPendingInteractions` (`agent runtime conflict`). The H2 worker confirmed its focused tests pass. No shared Docker integration test was run for this Task.
- The test compiler emitted the existing linker warning `ignoring duplicate libraries: '-lc++'`; it did not affect focused test exits.

## Scope and checkpoint integrity

Owned source files: `internal/container/container.go` and `internal/container/craft_runview_assembly_test.go`. `container.go` has concurrent unrelated edits from other tasks in this shared worktree; the task-local patch includes only the R5 registration and assembly hunks plus the new test file. `git apply --reverse --check` passed for that task-local patch before the report was written.

Checkpoint JSON: `docs/plans/2026-09-24-craft-107-runview-r5-engine-task2-checkpoint.json`.  
Task-local patch: `docs/plans/2026-09-24-craft-107-runview-r5-engine-task2-task-local.patch`.

Task source hashes at checkpoint:

- `internal/container/container.go`: `38eda3be11864e57946f108b0d1a498271ae182936f9817f4f3e9d66f0f230e3` (whole shared file; patch isolates this Task's hunks).
- `internal/container/craft_runview_assembly_test.go`: `5e6ac92c097a41ccf11b2f06a77d149aa5c546186e3b6bf3c67dcf8f53ac98c2`.
- Task-local patch: `c7491ddb7174691388f218428f2685110e1b08367fefef8d09847a4fc1e887a5`.

## Remaining gate

The production registry digest is still absent in `docker/craft/opencode.lock.json`; therefore production assembly remains unavailable until a verified digest and matching server pins are supplied. H3 must consume `CraftRunViewProductionAssembly` and handle its unavailable state before any production enablement.
