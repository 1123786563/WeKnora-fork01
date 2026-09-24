# Craft #107 T01 RunView R5 engine Task 2 Fix1 report

Checkpoint: `t01-r5-engine-task2-fix1-authority-guard-1`  
Workspace: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`  
Base: `a5e9195acd6500c085c85d60c852148e7bbbbf34`  
Commit: none.

## Finding addressed

`ResolveMaterial` now injects the registered `*repository.AgentRunStore` and loads the current durable Run before calling `CraftRunViewRuntimeCoordinator.Resolve`. It rejects malformed Task identity (scope/fence tenant mismatch, empty owner/session/Run/workspace or missing worker/epoch), Run identity mismatches, an actor that differs from the authenticated `types.Caller`, a non-running or expired writer lease, a different worker owner or epoch, and a malformed or Workspace-mismatched immutable Run snapshot. The snapshot must parse through `service.ParseDurableRunSnapshot` and contain a Craft Workspace seed matching the Task's Workspace.

These checks run before coordinator access. Deny tests cover **15** stale/inconsistent inputs and assert zero RunView Store allocate/load, Docker engine, and OpenCode session calls. Valid current-run material resolution remains covered, and default-off behavior still refuses assembly while the registry digest is absent. No old `workDir` fallback was added.

## Verification

- `go test ./internal/container -run '^TestCraftRunViewProductionAssembly' -count=1` — PASS (includes valid resolution, unbound denial and all 15 preflight denial cases).
- `go test ./internal/container -run '^TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity)' -count=1` — PASS.
- `git diff --check` — PASS.
- `git apply --reverse --check docs/plans/2026-09-24-craft-107-runview-r5-engine-task2-fix1-task-local.patch` — PASS.
- Both test commands emitted only the existing linker warning `ignoring duplicate libraries: '-lc++'`.
- An earlier run exposed a nil test-table mutator panic in the new changed-caller case. The test now guards the optional mutator, and the final focused run above passes.

The container-wide suite was not rerun for Fix1. The prior coordinated full-container run was already red in two unrelated assembly tests; concurrent R4 and T19 work also made the shared package tree unstable during this Fix1 verification window.

## Exact checkpoint

- Previous Task2 checkpoint: `docs/plans/2026-09-24-craft-107-runview-r5-engine-task2-checkpoint.json` (`container.go` preimage SHA-256 `38eda3be11864e57946f108b0d1a498271ae182936f9817f4f3e9d66f0f230e3`; assembly test SHA-256 `5e6ac92c097a41ccf11b2f06a77d149aa5c546186e3b6bf3c67dcf8f53ac98c2`).
- Postimages: `docs/plans/2026-09-24-craft-107-runview-r5-engine-task2-fix1-postimage/`.
- Task-local cumulative patch: `docs/plans/2026-09-24-craft-107-runview-r5-engine-task2-fix1-task-local.patch`.
- Checkpoint manifest: `docs/plans/2026-09-24-craft-107-runview-r5-engine-task2-fix1-checkpoint.json`.

The complete prior `container.go` file content was not separately archived before Fix1; its verified preimage hash is preserved in the previous checkpoint. The prior Task2-owned source patch and exact assembly-test preimage are retained under `docs/plans/2026-09-24-craft-107-runview-r5-engine-task2-fix1-preimage/`. The cumulative task patch excludes unrelated shared `container.go` changes.

## Remaining gates

The server lock still has no verified registry image digest, so the production assembly remains unavailable by design. H3 integration and production enablement remain separate gates.
