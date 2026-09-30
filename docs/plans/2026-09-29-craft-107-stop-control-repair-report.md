# Task 1 Report — F17/F20 Run stop control

Status: implementation and focused verification complete; uncommitted checkpoint. No staging or commit.

## Scope and outcome

Implemented current Task role authorization for Run control and an atomic repository fence between accepted StopIntent and fresh delegation preparation. Container wiring injects the live Craft TaskAccessChecker. Request storage scope remains the Task owner's Session scope; authenticated identity comes from `types.CallerFromContext`. TaskRead (Owner/Collaborator/Viewer) may query status; TaskWrite may stop; owner can stop any Task Run; collaborator can stop only a Run whose ActorUserID matches their caller. Nonmembers and viewers cannot stop. Run identity must match tenant, session, and owner-scoped storage identity.

`PutStopIntent` now locks/validates the exact Run row before the stop-intent row; `PrepareTask` owns that same Run lock before checking the exact tenant/session/Run intent. Only typed `craft.ErrNotFound` allows fresh preparation; storage errors fail closed. Already-prepared identical delegation reuse remains available before this fence. The supported guarantee is only that no fresh PrepareTask commits after Stop acceptance; a provider POST for a delegation prepared before Stop can still occur. PostgreSQL execution evidence is unavailable because `TRPC_TEST_POSTGRES_DSN` was unset.

## Exact owned files and SHA-256

- `internal/application/service/craft_control.go` — `fa3c0ec17d798e7f01fb3a55422471058ff559fd72214a74559479cec0536dc8`
- `internal/application/service/craft_control_f17_f20_test.go` — `9a1219dcdbd1963d2fec955f60c04e5411e1db1993f589217071c08a2f833b1f`
- `internal/application/repository/craft_stop_intent.go` — `ab1c5ae98ef8deeb2cb0f5571b68fdcb027898e5dba7e0d9ae9a30444c60eb3b`
- `internal/application/repository/craft_stop_intent_t20_test.go` — `de572f9f7701ce9da7a13ebbae73103fbb8e6ab68b72faad6c4b69a82ffbc010`
- `internal/application/repository/craft_workspace.go` — `d1aa72c7da545c9f8dd441cfebf78422c36cf62398326a34bb1ffa609c6f25f7`
- `internal/application/repository/craft_workspace_f17_f20_test.go` — `5479e1ca03d6c98db9a884b68d5dab70a0c5e4b0e0206714682d8fabb0105962`
- `internal/container/craft_interaction.go` — `47626cd038e26d699dbc0e824b16335e80c3af267a975a167fdb27c6e73d830f`
- `internal/container/craft_interaction_wiring_test.go` — `206cb32d5e79a999e6d3776db3d9d57417b2d1f2c247cdd9412156ddd83abb25`

Initial and final HEAD: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159` (unchanged). Aggregate task patch (tracked changes plus both added test files, binary diff concatenation) SHA-256: `e37d84d739c4a1f379e8ad4963f34f0e05b53c767689517448b5648fae6d72ac` (28,344 bytes). Pre-existing untracked plan `docs/plans/2026-09-29-craft-107-stop-control-repair-plan.md` was preserved and is not part of this task's owned delta.

## TDD and verification evidence

RED before implementation:
- `go test ./internal/application/service -run 'TestCraftControlCollaboratorMayStopOnlyTheirOwnRun|TestCraftControlTaskRolesGateStopAndStatus' -count=1` — failed as expected: collaborator B and Viewer were accepted by old Stop path.
- `go test ./internal/application/repository -run 'TestCraftPrepareTaskStopIntent|TestCraftPrepareTaskSameSessionDifferentRunAfterPriorStop|TestCraftStopIntentAndPrepareTaskSerializeOnRun|TestCraftStopIntentStoreRequiresExactRunIdentityBeforeAcceptingIntent' -count=1` — failed as expected: fresh preparation was not fenced by StopIntent and missing stop-intent storage was not fail-closed.

GREEN/final focused runs:
- `go test ./internal/application/service -run 'TestCraftControlCollaboratorMayStopOnlyTheirOwnRun|TestCraftControlTaskRolesGateStopAndStatus' -count=1` — PASS (`ok`, 10.007s).
- `go test ./internal/container -run 'TestNewCraftInteractionAssemblyInjectsCurrentTaskAccess|TestWireCraftInteractionRegistrarRegistersPendingInteractions' -count=1` — PASS (`ok`, 21.271s).
- `go test ./internal/application/repository -run 'TestCraftPrepareTaskStopIntent|TestCraftPrepareTaskSameSessionDifferentRunAfterPriorStop|TestCraftStopIntentAndPrepareTaskSerializeOnRun|TestCraftStopIntentStorePersistsAcrossReconstruction|TestCraftStopIntentStoreConcurrentConfirmNeverDowngrades|TestCraftStopIntentStoreRequiresExactRunIdentityBeforeAcceptingIntent' -count=1` — PASS (`ok github.com/Tencent/WeKnora/internal/application/repository 82.657s`). This is the final run after the last source edits.
- `git diff --check` — PASS (exit 0).

PostgreSQL DSN check: `TRPC_TEST_POSTGRES_DSN` unset; PostgreSQL-backed runtime behavior was not exercised. SQLite tests exercised exact Run scoping, same-session different Run independence, storage-error refusal, and a deterministic concurrent Run-lock ordering scenario.
