# T01 RunView R4 Task 1 Fix1 — task report

## Status

**Implemented; pending independent review and separately owned H2 fixture correction.** This Fix1 addresses the independent review's findings 1–4. It does not implement R4 Task 2, alter H2/R5-owned tests, or enable runtime assembly.

## Changes

- Added a draft-object reader bounded by the declared file size plus one byte, with a hard 50 MiB cap, exact byte count, SHA-256 verification, reader close handling, and cancellation check. The existing 20 MiB input cap no longer truncates legal draft files.
- Hardened output-tree inspection: after every `Fstatat` preflight, `Openat` descriptors are checked with `Fstat` for device/inode, type, mode, link count, and size. File path identity is rechecked after the read; intermediate directory descriptor and path identities are checked before and after recursive inspection. Deterministic test hooks exercise replacement between stat and open.
- Rechecked durable Run epoch after materialization, after provider revalidation immediately before dispatch, and on entry to message-ID adoption. Adoption now updates only when the durable tenant/Run/session/owner/epoch still matches and the delegation row also matches Workspace identity; stale retries fail without mutation.
- Added production `Execute` boundary tests using a scoped SQLite fixture and verified material handle. They cover explicit empty revision zero, frozen D1 when current Workspace head is D2, retry of identical bytes, 21 MiB accepted draft, >50 MiB rejected before object fetch, wrong Run/Workspace/epoch, mismatched object bytes, epoch changes during fetch, epoch changes during final material revalidation, and stale duplicate message-ID adoption. Filesystem tests cover same-byte hard-link replacement and intermediate directory replacement after preflight.

## RED/GREEN evidence

- RED `go test ./internal/container -run '^TestCraftRunViewR4Execute' -count=1` failed as expected for the 21 MiB file (`declares 22020096 bytes but stored 20971521`) and for epoch mutation during object fetch (expected conflict, got nil).
- RED descriptor replacement tests failed to compile because the `inspectRunViewDraftOutputWithOps` seam was not yet defined.
- GREEN `go test ./internal/container -run '^TestCraftRunViewR4' -count=1` — PASS.
- GREEN `go test -race ./internal/container -run '^TestCraftRunViewR4' -count=1` — PASS.
- `go test ./internal/container -count=1` — FAIL with four classified failures:
  - `TestCraftAccessFeatureRegistriesAreAssemblyOwned`: missing `craft.TaskAccessChecker` dependency.
  - `TestWireCraftInteractionRegistrarRegistersPendingInteractions`: `agent runtime conflict`.
  - `TestLocalCraftRuntimeRejectsKnowledgeMutationAfterResolver/changed_published_bytes_after_resolver`: fixture's fence is incomplete (`Fence.Epoch` is zero), so the new writer-fence gate returns `craft forbidden: incomplete durable Run writer fence` before its expected verifier failure.
  - `TestLocalCraftRuntimeAcceptsIdenticalKnowledgeRetryAndRestart`: same zero-epoch fixture failure.

The two assembly failures are unrelated to this task. The two H2 test fixtures are owned by R5 and are intentionally untouched; their current schema also lacks `agent_runs.epoch`, and their replacement snapshot does not include `craft_workspace_seed`. R5/task-parent must correct the fixture with a realistic positive epoch and explicit seed once ownership is released. Runtime keeps strict epoch/seed checks and provides no fallback.

- `gofmt -d internal/container/craft_runtime.go internal/container/craft_runtime_r4_seed_test.go internal/container/craft_runtime_r4_seed_execute_test.go` — clean.
- `git diff --check` — PASS.

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit created.
- Fix1 task-start hashes: runtime `49b2650f5d6b2e851a0ef46065f691d93012c5a174f282f8aa34bc64ee153000`; seed tests `491a277d0011f6f6e3e79a89e10f5c0673e513e118fc09920cea7b73b5014665`; execute-boundary test file absent.
- Current runtime SHA-256: `8ca77bf0a1d8dbb93c33eb574166f3ed7e66e73c206c330d3f2ec5cdc67469ee`.
- Current `craft_runtime_r4_seed_test.go` SHA-256: `82a4403f2ba1d7d98408a100c872e5f47ab39c8fd5a866b1f35f0585664687ee`.
- Current new `craft_runtime_r4_seed_execute_test.go` SHA-256: `542afeba2b16323a5317340655ced033924fcdc02d44a60130e1b490a4e724f5`.
- Fix1-only patch: `docs/plans/2026-09-24-craft-107-runview-r4-task1-fix1-task-local.patch`, SHA-256 `150f672922291f1c543db53889e394512364341faab8764357400cfbd7303d02`. Its baseline runtime/test files reconstruct the previous Task1 checkpoint hashes above.

## Remaining

- Independent Fix1 Spec/quality review is pending.
- Task 2 remains separately gated. H2 fixture correction remains outside my ownership until R5 releases it.
