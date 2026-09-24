# T08 durable actor Task 3 fix 1 report

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit created)
- Task-local result is uncommitted. The worktree contains concurrent edits from T01, T05, T19, RunView, and frontend tasks; these were preserved. `container.go` was not edited.
- Runtime did not expose actual agent role/model/reasoning metadata; none is inferred here.
- `git diff --check`: passed.

## Implementation

Task 1 adds a typed `CraftTaskLookup` and composite `TaskRunAccess` seam. The existing persistent Craft access service classifies a task using tenant/session-scoped `sessions` + `craft_sessions` registration, independent of grant lookup. Session service DI reuses the existing provider. Durable run execution and follow-up now classify before owner fallback: a lookup error fails closed, registration/marker disagreement fails closed, and Craft rows require an immutable actor plus current TaskWrite. Generic legacy rows retain owner fallback only when the registration lookup authoritatively says the task is not Craft.

Task 2 adds two linked behavior tests. The graph-level test proves restored collaborator C is passed through `ToolExecContext` into the real `AppConnectorTool` subject constructor. The connector service test exercises real ActionService, connection/action storage, and A02 guard: owner-private action is denied with zero dispatch calls, while the collaborator's own permitted connection dispatches once with collaborator C in the dispatcher snapshot. These are linked seam tests, not one end-to-end durable-run-to-production-facade test.

The approved fixture-only change in `craft_delegate_test.go` injects the registered TaskRunAccess checker into the existing delegation fixture. It does not change production behavior.

## TDD and verification evidence

RED before Task 1 implementation:

```text
go test ./internal/application/service -run '^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor$' -count=1
```

Failed as intended: expected `craft.ErrForbidden`, got nil for an unmarked historical Craft snapshot with a null actor. The regression test includes an Owner with live TaskWrite, ensuring the prior owner fallback would otherwise have authorized it.

Task 1 focused GREEN:

```text
go test ./internal/application/service -run '^(TestCraftTaskLookupIsTenantAndSessionScopedIndependentOfGrant|TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor|TestDurableRunClassificationLookupFailureAndManifestDisagreementFailClosed|TestExecuteDurableGenericLegacyRunRetainsOwnerFallback|TestExecuteDurableCraftRun(RestoresActorAcrossWorkerLeaseRecovery|RejectsLegacyMissingActorBeforeCapabilities|RechecksTaskWriteBeforeModelResolution))$' -count=1
```

Passed (6.607s).

Combined verification:

```text
go test ./internal/application/service -run '^TestExecuteDurable' -count=1
```

Passed (67.905s).

```text
go test ./internal/application/service -run '^TestCraftTaskLookupIsTenantAndSessionScopedIndependentOfGrant$|^TestDurableRunClassificationLookupFailureAndManifestDisagreementFailClosed$|^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor$|^TestExecuteDurableGenericLegacyRunRetainsOwnerFallback$|^TestDurableCraftActorFlowsThroughAppConnectorToolSubject$' -count=1
```

Passed (15.176s).

```text
go test ./internal/modules/appconnector/service/appconnector -run '^TestActionServiceRechecksPersonalConnectionAgainstPersistedCraftActor$' -count=1
```

Passed (2.686s).

```text
go test ./internal/container -run '^TestCraftAccessFeatureUsesPersistentPolicyOnConstrainedRoutes$' -count=1
```

Passed (5.527s).

After the final test fixture adjustment to use the real Craft access service and verify Owner TaskWrite, rerun:

```text
go test ./internal/application/service -run '^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor$|^TestDurableCraftActorFlowsThroughAppConnectorToolSubject$' -count=1 && git diff --check
```

Passed (4.887s); diff check passed.

## Owned-file hashes

SHA-256 of complete final file contents at this checkpoint:

| File | Before | After |
|---|---|---|
| `internal/modules/craft/web_contracts.go` | `8fa39b494b4a8dc26a44aadcc540a5e346bd7bf4aa667035df5c59bf61519728` | `effbfcfff1ff261c66da9a6b3367b13e1aa06fab2cae995ffaedb1f7742a4bbc` |
| `internal/application/service/craft_access.go` | `ab524813c5a71cba105372f1391e5e75f9296022d6c6e0dc79c9d483e7714721` | `51361a099ef5111d895c73414a112629cd4a31af51c2661587d487ceb8f25eca` |
| `internal/application/service/craft_access_test.go` | `40b5e0f3bbdde648832a232027f7c6b0cb5b24ee56c4998769b042cc790ddb71` | `9c696fb14695e370937a8f9d6379d58bf0918032a1f7e411a02760610965effe` |
| `internal/container/craft_access_wiring.go` | `e0f887826e0938f6d5aef3824336d7ab50759f2ae910ab68fe6cbf0748a5fa56` | `86db3aed871adeabb68075832b9a18bcdd1ebe101c6f9004e571b3d3b052c177` |
| `internal/application/service/session.go` | `49592b157b42337fc2f5fcbbc0d0986bf5a6233cb145ccf22b69ca97f6a235ab` | `1b47ca484e99fda16fedd32a237df2f2453dbcffcc2ffb3dec25576547f8740e` |
| `internal/application/service/agent_run_graph.go` | `537bd72a0f2f03f8ead3105f72a2451520be8a289f4eabd347181aa1e628edde` | `5bc1a7a7ddc882ee5b12dd068fbd96fa4c32c78ba0ef5c106288d413de90e4d0` |
| `internal/application/service/agent_run_graph_test.go` | `7bb78800f00669d84dd7a7ce36d4cde0ee521a75f44c03d3f77685cbb099ffca` | `e1ffc73d394e86a1e6a0a934e5032cf273aa37d6262ac3b42318d1e3096d3b1b` |
| `internal/application/service/craft_delegate_test.go` | `5269ff8f24b788760458f8ccd1b12f399986f5b10ad67b945e1af4bc28c0f0a0` | `9fc96eec5147df2ba3dc58b0515ec59139133e7554276b5c66f3a074378329b9` |
| `internal/modules/appconnector/service/appconnector/action_test.go` | `bd2552eec6386dc3a9f618b19b3cec3d800ea408b769a2f6d1a05113cee35404` | `fe840c61d7c33222e1285b310efeb0de832148e6948f79e6a57fcf53bf86c6f1` |

The before hashes for the first seven files identify the pre-task contents observed at this round's start; their files already included concurrent or earlier T05 changes. The two Task2 test files were clean at task start, so their before hashes are the current HEAD contents. The after hashes bind the listed tests and this report to the same source checkpoint; re-hash before review if any shared-worktree edits continue.

## Remaining gates and limitations

- Independent review of this fix is pending.
- This does not establish the central T05 production Publisher, per-Run read isolation, or dispatch-time knowledge/source revalidation.
- Connector evidence composes two focused seams; the complete graph-to-production-facade path was not assembled in one integration test.
- No commit was made.
