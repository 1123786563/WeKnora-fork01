# T01 RunView R4 Task 1 — task report

## Status

**Initial task state: BLOCKED before implementation.** The durable admission snapshot freezes the predecessor identity, but the available typed `craft.DraftHeadStore` only exposes `Read(ctx, scope, workspaceID)`, which returns the current head. It has no exact immutable-revision lookup for resolving the files belonging to the frozen `draft_revision` when the current head has since advanced. The Task 1 requirement explicitly forbids substituting the current head, a Version, or a prior Run for the frozen predecessor.

The parent then assigned the prerequisite plan `2026-09-24-craft-107-draft-revision-read-seam-plan.md`; that seam is implemented and reported separately in `2026-09-24-craft-107-draft-revision-read-seam-report.md`. R4 Task 1 runtime work remains gated on independent review of that seam.

I reported the missing seam to the parent before widening ownership. Smallest required interface:

```go
ReadRevision(ctx context.Context, scope craft.Scope, workspaceID string, revision int64) (craft.DraftHead, error)
```

The repository implementation must scope authorization to the tenant/owner/session Workspace, load that exact immutable revision and its files, and validate source Run, manifest digest, canonical paths, sizes, and file references. Runtime must compare the result to every field in the server-only `craft_workspace_seed` stored on the durable Run before materialization. Until this seam is assigned and available, no seed path can safely satisfy the brief.

## Baseline and ownership

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD at task start: `a5e9195acd6500c085c85d60c852148e7bbbbf34`
- `internal/container/craft_runtime.go` was already modified in the shared worktree before this task (`628` insertions, `57` deletions relative to HEAD). Its task-start SHA-256 is `1d37595229ae07699af052f4fb0ca1c2f238c36054ccc57db7c986941bd2e4c2`.
- No production or test source files were changed by this task. No task-local patch was produced. Existing H2 tests and T19 edits were left untouched.

Task-start hashes:

| File | SHA-256 |
| --- | --- |
| `internal/container/craft_runtime.go` | `1d37595229ae07699af052f4fb0ca1c2f238c36054ccc57db7c986941bd2e4c2` |
| `internal/container/craft_runtime_test.go` | `e93add4c5ba830050cce06708ac595ac540537be05141d8e606e1effddc37f15` |
| `internal/modules/craft/draft_head.go` | `ae779d96fd98d66fc42cc637da9a48f45cf791935c0d12775eddcda950f69473` |
| `internal/application/repository/craft_draft_head.go` | `d85f9c69394255e79edd8905c209b6d2d4961fc9a9b3c2535250ed1183897b3b` |

## Verification

- `go test ./internal/modules/craft ./internal/application/repository -run 'DraftHead' -count=1` — PASS (`craft` 0.648s; `repository` 8.119s). This confirms the existing draft-head contract tests only; it does not resolve the missing exact-revision lookup.
- Runtime seed and filesystem rejection tests were not added or run because the required exact immutable-revision file-access seam is absent. Implementing through current-head lookup would violate frozen predecessor, retry, and later-revision invariants.

## Remaining work

After the typed exact-revision read seam is added under its owner and independently reviewed, resume Task 1: parse the durable Run's server-only seed identity, resolve and compare that exact revision, verify pinned object bytes and safe output publication under the verified material handle, and add uniquely named focused tests. Do not enable runtime assembly or implement Task 2 here.

## R4 Task 1 implementation checkpoint

**Status: implemented, pending independent review.** The previously missing immutable revision seam is now available and was independently reviewed PASS. The typed `service.ParseDurableRunSnapshot` schema also now carries and validates `craft_workspace_seed`; no second parser or widened service ownership was added.

### Changes

- `localCraftRuntime.Execute` seeds the generation-private `output/` directory from the seed frozen in the admitted durable Run, before dispatch. The prior `pointWorkspaceOutput` call is no longer used by this Task 1 path.
- The seed loader scopes the durable Run lookup by tenant, Run, session, and owner; checks the task fence epoch against the persisted Run epoch; verifies material key scope and Workspace identity; and resolves the exact revision with `CraftDraftHeadStore.ReadRevision`.
- Runtime compares Workspace ID, revision, state, source Run, and manifest digest with the frozen seed. Missing, malformed, unknown, stale, or foreign predecessors fail closed. Explicit revision zero is accepted only as an explicit empty head.
- Selected files are validated for canonical paths, duplicate and file/directory-collision paths, per-file read cap, aggregate draft quota, object byte length, and SHA-256. Only referenced objects are fetched.
- Seed publication uses the verified material layout, directory/file descriptors with `O_NOFOLLOW`, private output-root identity, regular-file and single-link checks, same-directory temporary files and no-replace atomic rename. Existing exact files are idempotently accepted; mismatched, extra, unsafe, and incomplete trees fail closed. Nested directory entries are fsynced. No Run A, HOME, inputs, or knowledge tree is copied.
- Added uniquely named focused tests for empty/selected/frozen/moved/unknown seed metadata, path collisions, exact retry bytes, changed bytes, symlinks, and unpinned output entries.

### Verification

- `go test ./internal/container -run '^TestCraftRunViewR4' -count=1` — PASS.
- `go test ./internal/application/repository -run '^TestCraftDraftHeadReadRevision' -count=1` — PASS.
- `go test ./internal/application/service -run 'DurableRunSnapshot|CraftWorkspaceSeed' -count=1` — PASS.
- `go test ./internal/container -count=1` — FAIL with four failures. Two unrelated assembly failures: `TestCraftAccessFeatureRegistriesAreAssemblyOwned` reports missing `craft.TaskAccessChecker`; `TestWireCraftInteractionRegistrarRegistersPendingInteractions` reports `agent runtime conflict`. Two existing H2 runtime failures are in `TestLocalCraftRuntimeRejectsKnowledgeMutationAfterResolver/changed_published_bytes_after_resolver` and `TestLocalCraftRuntimeAcceptsIdenticalKnowledgeRetryAndRestart`: their fixture query fails with `no such column: epoch`. The H2 fixture builds a snapshot through `BuildDurableCraftRunSnapshotWithKnowledgeSelection`, which omits `craft_workspace_seed`, and its `agent_runs` setup also lacks the durable `epoch` column. The runtime intentionally keeps the epoch check and has no fallback; parent plans a serial fixture correction after its owner finishes.
- `gofmt -d internal/container/craft_runtime.go internal/container/craft_runtime_r4_seed_test.go` — clean.
- `git diff --check` — PASS.

### Checkpoint identity

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit).
- Reconstructed task-start runtime preimage hash matches the recorded task-start hash: `1d37595229ae07699af052f4fb0ca1c2f238c36054ccc57db7c986941bd2e4c2`.
- Current `internal/container/craft_runtime.go`: `49b2650f5d6b2e851a0ef46065f691d93012c5a174f282f8aa34bc64ee153000`.
- Current new `internal/container/craft_runtime_r4_seed_test.go`: `491a277d0011f6f6e3e79a89e10f5c0673e513e118fc09920cea7b73b5014665`.
- Task-local patch: `docs/plans/2026-09-24-craft-107-runview-r4-task1-task-local.patch`, SHA-256 `fb4d0b9b793eaa8c9005b4c7a584d164d1d7fc4c5e1e14d2e24fd69f8f875ed4`. Its runtime preimage reconstruction exactly reproduces the task-start hash; it includes only this Task 1 delta and the new focused test file.

### Remaining

- Independent Task 1 Spec and code-quality review is pending.
- H2 fixtures need their separately owned update to include valid `craft_workspace_seed` and `epoch` schema/data before runtime integration tests can pass. No existing H2 test file was modified.
- Runtime remains default-off. Task 2 output capture/collection and central assembly remain out of scope.
