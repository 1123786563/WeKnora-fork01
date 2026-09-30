# T01 RunView R4 Task 1 Fix2 — implementation report

## Scope

Implemented Fix2 in `internal/container/craft_runtime.go` and `internal/container/craft_runtime_r4_seed_execute_test.go` only. The existing H2/R5 fixtures and other workers' changes were not edited. HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit was created.

## Changes

- Message-ID adoption now loads the exact tenant/Run/tool-call/Workspace delegation and compares the immutable request fields before modifying anything. The identity comparison follows `CraftStore.PrepareTask`: request, scope, Run key, inputs, skills and deadline must match; a worker owner/epoch and prompt-message-ID may change during recovery.
- Adoption changes the message ID with a CAS over the stored `task_json`, expected old message ID, exact delegation identity, and current tenant/session/owner/epoch Run fence. It preserves a matching existing `msg_` ID. After a lost CAS, it reloads and adopts only a message ID backed by the same request and current fence. SQLite shared-cache lock-upgrade errors are retried in a bounded, context-aware loop only while the expected old row remains unchanged.
- Changed request collision is exercised through `Execute` and is rejected before `inner.Execute` (prompt dispatch); the durable task JSON and old ID remain unchanged.
- Execute-boundary tests cover two concurrent adopters converging on one ID, identical retry, changed request, completion of a partial frozen D1 seed, changed private bytes rejecting retry before dispatch, and failed B leaving persisted V1 as the session's newest/current version with V1's manifest and file bytes unchanged.

The version service has no separate mutable “default pointer”: `CraftSessionService.View` projects the first row from `CraftVersionStore.List` (newest first). The failed-B test observes this persisted service seam; V1 is the only/current row before and after the failure.

## TDD evidence

- Before the implementation change, the same-request retry regression failed because it minted a replacement message ID (`TestCraftRunViewR4MessageAdoptionSameRequestRetryReusesPersistedID`).
- Before the implementation change, the changed-request collision regression returned no error and overwrote the durable task request (`TestCraftRunViewR4MessageAdoptionRejectsChangedRequest`).
- An initial concurrent regression run exposed SQLite `database table is locked`; the bounded CAS reload/retry path now handles the loser without adopting an unpersisted ID.
- Partial-output replay and failed-B/V1 tests exercise already existing seed/failure behavior through production `Execute`; these cases required acceptance tests but no additional mutation of the seed or version-publish path.

## Verification

| Command | Result |
| --- | --- |
| `go test ./internal/container -run '^TestCraftRunViewR4ConcurrentMessageAdoption' -count=50` | PASS |
| `go test ./internal/container -run '^TestCraftRunViewR4' -count=1` | PASS |
| `go test -race ./internal/container -run '^TestCraftRunViewR4' -count=1` | PASS |
| `gofmt -d internal/container/craft_runtime.go internal/container/craft_runtime_r4_seed_execute_test.go` | PASS (no output) |
| `git diff --check` | PASS |
| `go test ./internal/container -count=1` | FAIL in four unrelated/pre-existing cases described below |

The full container failures were:

1. `TestCraftAccessFeatureRegistriesAreAssemblyOwned`: missing `craft.TaskAccessChecker` in assembly fixture.
2. `TestWireCraftInteractionRegistrarRegistersPendingInteractions`: `agent runtime conflict` in the assembly fixture.
3. `TestLocalCraftRuntimeRejectsKnowledgeMutationAfterResolver/changed_published_bytes_after_resolver`: R5 H2 fixture stops at `craft forbidden: incomplete durable Run writer fence` because its task has no epoch.
4. `TestLocalCraftRuntimeAcceptsIdenticalKnowledgeRetryAndRestart`: the same missing durable `Fence.Epoch`; fixture schema/snapshot also predate the current Run epoch and `craft_workspace_seed` requirements.

The H2/R5 tests remain untouched and require their separately owned fixture correction. The two assembly failures are outside Fix2 ownership. They are not treated as passing evidence.

## Checkpoint

- Checkpoint JSON: `docs/plans/2026-09-24-craft-107-runview-r4-task1-fix2-checkpoint.json`.
- Patch artifact: `docs/plans/2026-09-24-craft-107-runview-r4-task1-fix2-task-local.patch`.
- The patch artifact is a complete owned-file post-image snapshot. The pre-existing Execute test was untracked at Fix1 start and its preimage content was not persisted, so a reconstructable line-delta patch against that exact file is unavailable; its preimage SHA is recorded in the Fix1 checkpoint. The runtime preimage is likewise recorded there, and this checkpoint records both pre/post hashes.

## Remaining review risks

- `adoptExecutorMessageID` has direct concurrent adapter coverage on SQLite and repeated race tests; the production database's concurrent transaction behavior should still be independently reviewed.
- “Current/default V1” is derived from newest-first version listing rather than an explicit pointer column; this test follows the existing `CraftSessionService.View` contract.
- Full-package red remains until separately owned assembly/H2 fixtures are corrected.
