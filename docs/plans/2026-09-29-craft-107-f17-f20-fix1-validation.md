# F17/F20 Fix1 Independent Validation

**Status:** PASS for the focused service, HTTP, repository, and container selectors; PostgreSQL runtime coverage unavailable. Source and test files were not modified by this validation.

## Revision and frozen input

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-stop-fence/WeKnora-fork01`
- HEAD: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159` (Fix1 is an uncommitted checkpoint on this base).
- Task report: `docs/plans/2026-09-29-craft-107-f17-f20-fix1-report.md`.
- Patch: `.superpowers/sdd/2026-09-29-craft-107-f17-f20-fix1/review-package/fix1.patch`, SHA-256 `204c26d2cd64f2ac4117f7663ebae5700715d3d7d2e8be35561317207fbc090f` (matches the report and package manifest).
- Final source hashes checked against report and package `after/` manifest:
  - `internal/application/service/craft_control.go`: `f5a8526c1aaa23c3ee70f91f33ba1dbaa4220ffb6cc55b59d42b18bd520a72ef`
  - `internal/application/service/craft_control_f17_f20_test.go`: `de17056ac8cf60403b57bf437b4308f972150601bf96ec4f11fda85faca2ee3c`
  - `internal/handler/session/craft_interaction_test.go`: `f385f40f3948030bc0549f36b3733d115490082bdf683efc6df78683767fb5c5`

## Commands and results

All commands ran from the worktree above after the source hashes were frozen.

- `go test ./internal/application/service -run 'TestCraftControlRequiresTaskAccessChecker|TestCraftControlRejectsDelegationFromAnotherRun|TestCraftControlCollaboratorMayStopOnlyTheirOwnRun|TestCraftControlTaskRolesGateStopAndStatus' -count=1` — PASS (`ok`, 7.336s).
- `go test ./internal/handler/session -run TestStopCraftRunCollaboratorUsesPersistedOwnerStorageScope -count=1` — PASS (`ok`, 20.668s).
- `go test ./internal/application/repository -run 'TestCraftPrepareTaskStopIntent|TestCraftPrepareTaskSameSessionDifferentRunAfterPriorStop|TestCraftStopIntentAndPrepareTaskSerializeOnRun|TestCraftStopIntentStorePersistsAcrossReconstruction|TestCraftStopIntentStoreConcurrentConfirmNeverDowngrades|TestCraftStopIntentStoreRequiresExactRunIdentityBeforeAcceptingIntent' -count=1` — PASS (`ok`, 79.484s).
- `go test ./internal/container -run 'TestNewCraftInteractionAssemblyInjectsCurrentTaskAccess|TestWireCraftInteractionRegistrarRegistersPendingInteractions' -count=1` — PASS (`ok`, 16.386s); linker emitted non-fatal duplicate `-lc++` warning.
- `git diff --check` — PASS (exit 0).

## Acceptance evidence and limits

The selectors exercise fail-closed behavior when the access checker is unavailable, reject cross-Run delegation, enforce the role/actor stop rules, cover the collaborator HTTP journey while storing the stop intent in the persisted Run owner scope, test the serialized Run-lock stop-intent fence and exact identity selection, and confirm current TaskAccessChecker container injection.

`TRPC_TEST_POSTGRES_DSN` is unset. No PostgreSQL runtime behavior is claimed. These focused checks do not establish the broader Spec #107 acceptance suite or provider-side cancellation after a delegation was already prepared; the implementation report explicitly limits the Run-lock guarantee to denying a fresh `PrepareTask` after a committed Stop intent.
