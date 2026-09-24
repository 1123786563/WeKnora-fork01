# T08 actor Task 3 classification fix 2 report

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit)
- Only production/test files in the approved ownership were modified: `craft_access.go`, `craft_access_test.go`, and `agent_run_graph_test.go`. All pre-existing edits from fix1 and other agents were preserved.
- Runtime did not expose role/model/reasoning metadata; none is inferred.
- Exact task-local source patch: `docs/plans/2026-09-24-craft-107-t08-actor-task3-fix2-task-local.patch`, SHA-256 `d038a5f50809aeebabd00342a1688d83be3d5348422a5331161eb19d47c289de`.
- `git diff --check` for the owned source/test files passed.

## Change

`CraftAccessService.IsCraftTask` now performs two explicit lookups: first it proves an active Session exists within the requested tenant, then it checks tenant-scoped Craft registration. A missing, soft-deleted, or wrong-tenant Session returns `craft.ErrNotFound`, which the existing execution and follow-up paths map to fail-closed authorization. Only an active Session with no Craft registration returns `false, nil` and retains generic legacy behavior. The sequential checks avoid the former inner-join ambiguity.

Lookup tests cover active registered Craft, active generic, soft-deleted registered Craft, absent Session, and a Session that exists only in another tenant. A graph regression keeps a retained Craft registration and Owner TaskWrite grant, soft-deletes the Session while a Run lease exists, and verifies actorless execution is forbidden before model/capability work and that follow-up admission does not add a Run.

## RED → GREEN evidence

RED was run before the lookup implementation:

```text
go test ./internal/application/service -run '^TestCraftTaskLookupFailsClosedForMissingOrDeletedSession$|^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActorAfterSessionDelete$' -count=1
```

Failed as expected. Lookup subtests for deleted, missing, and wrong-tenant Sessions each received nil instead of `craft.ErrNotFound`. The graph regression returned `agent run lease lost` instead of `craft.ErrForbidden`, showing the inconsistent Session had passed the required pre-model classification gate.

GREEN focused service command:

```text
go test ./internal/application/service -run '^TestCraftTaskLookupIsTenantAndSessionScopedIndependentOfGrant$|^TestCraftTaskLookupFailsClosedForMissingOrDeletedSession$|^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActorAfterSessionDelete$|^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor$|^TestDurableRunClassificationLookupFailureAndManifestDisagreementFailClosed$|^TestExecuteDurableGenericLegacyRunRetainsOwnerFallback$' -count=1
```

Passed: `ok github.com/Tencent/WeKnora/internal/application/service 13.908s`.

Adjacent checks:

```text
go test ./internal/container -run '^TestCraftAccessFeatureUsesPersistentPolicyOnConstrainedRoutes$' -count=1
```

Passed: `ok github.com/Tencent/WeKnora/internal/container 9.115s` (linker emitted only the duplicate `-lc++` warning).

```text
go test ./internal/modules/appconnector/service/appconnector -run '^TestActionServiceRechecksPersonalConnectionAgainstPersistedCraftActor$' -count=1
```

Passed: `ok github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector 5.032s`.

## Owned-file content hashes

SHA-256 at task entry and at this checkpoint:

| File | Before | After |
|---|---|---|
| `internal/application/service/craft_access.go` | `51361a099ef5111d895c73414a112629cd4a31af51c2661587d487ceb8f25eca` | `b2ccaa8a34accd9ca3e888783b94c62330baddc669c1846f9052789d43c9d9f9` |
| `internal/application/service/craft_access_test.go` | `9c696fb14695e370937a8f9d6379d58bf0918032a1f7e411a02760610965effe` | `1e7954a0e6197b2198e51174387fc9e2cb3774530cb20f46a11d94d4aa2c4c27` |
| `internal/application/service/agent_run_graph_test.go` | `e1ffc73d394e86a1e6a0a934e5032cf273aa37d6262ac3b42318d1e3096d3b1b` | `6183f5359cdee61fc1f96994d3b22f19207dd1290ed5b3bf1b045236e1375194` |

The before hashes match the prior fix1 checkpoint and were verified against the reconstructed task-local patch baseline. The report and patch are uncommitted.

## Remaining gate

Independent re-review is pending. T05 graph ownership remains reserved until that review passes. This fix addresses only session classification; it does not claim broader T05 or full T08 completion.
