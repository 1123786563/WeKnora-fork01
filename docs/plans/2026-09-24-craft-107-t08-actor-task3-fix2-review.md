# T08 actor Task 3 classification fix 2: independent scoped re-review

Date: 2026-09-24. Read-only review of the fix 2 plan/report, its exact task-local patch, prior fix 1 review, approved Spec #107/T08, `CONTEXT.md`, and ADR-0004/0009. No source or test edits, OCR, or delegation. HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`; this is an uncommitted shared-worktree checkpoint.

## Checkpoint and verdict

The task-local patch SHA-256 is `d038a5f50809aeebabd00342a1688d83be3d5348422a5331161eb19d47c289de`, matching the report. Its three before hashes match the prior fix 1 reviewed final hashes. Current full-file after hashes also match the report:

| File | SHA-256 |
| --- | --- |
| `internal/application/service/craft_access.go` | `b2ccaa8a34accd9ca3e888783b94c62330baddc669c1846f9052789d43c9d9f9` |
| `internal/application/service/craft_access_test.go` | `1e7954a0e6197b2198e51174387fc9e2cb3774530cb20f46a11d94d4aa2c4c27` |
| `internal/application/service/agent_run_graph_test.go` | `6183f5359cdee61fc1f96994d3b22f19207dd1290ed5b3bf1b045236e1375194` |

**Scoped Spec compliance: PASS.** `IsCraftTask` now proves the active tenant/session row before querying Craft registration. Missing, deleted, and wrong-tenant Sessions return `craft.ErrNotFound`; execution maps that lookup error to `craft.ErrForbidden` before model/capability resolution, and follow-up admission returns without inserting a Run. Active generic Sessions alone can receive `(false, nil)` and retain the documented legacy owner fallback. The earlier High classification finding is resolved at this checkpoint.

**Scoped quality: FAIL on one Medium test coverage finding below.** The production change is narrow and the focused checks pass, but fix 2 removed the active unmarked-Craft regression while adding the deleted-Session variant. The explicit plan called for both cases.

**T05/full T08: NOT VERIFIED.** This review does not validate source-record rechecks or the broader T08 gates.

## Finding

### Medium — active unmarked Craft regression was replaced, not retained

**Evidence / affected test:** The task-local patch renames `TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor` to `TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActorAfterSessionDelete` at `agent_run_graph_test.go:467` and adds a soft delete before worker execution at `:489-490`. No test with the old name remains. The reported GREEN command includes the old name as a regex alternative, but it matches no test. Other actorless Craft tests use the new marked snapshot, so the active historical unmarked case from fix 1 no longer has a worker-level regression.

**Impact:** A later change that classifies active Craft solely from the new manifest could pass this focused suite while reintroducing the original owner fallback. The current production code does not exhibit that defect.

**Smallest defensible correction:** Keep the original active unmarked Craft test and add the soft-deleted variant as a separate case or table subtest, preserving the no-model/no-capability/no-follow-up assertions for both. Use anchored test listing or separate invocations so a stale regex alternative cannot silently pass.

## Verification and limits

I independently ran `go test ./internal/application/service -run '^TestCraftTaskLookupIsTenantAndSessionScopedIndependentOfGrant$|^TestCraftTaskLookupFailsClosedForMissingOrDeletedSession$|^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActorAfterSessionDelete$|^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor$|^TestDurableRunClassificationLookupFailureAndManifestDisagreementFailClosed$|^TestExecuteDurableGenericLegacyRunRetainsOwnerFallback$' -count=1`; it passed, but the old-name alternative is absent and contributed no executed test. `git diff --check` on the three owned files passed. The fix report records passing adjacent container and connector checks at the same source hashes; I did not repeat those unaffected package tests. The deleted-Session graph test keeps a live lease, an unmarked actorless Craft snapshot, retained registration, and Owner TaskWrite before deletion; it asserts `craft.ErrForbidden`, zero model/capability work, and no follow-up Run. The lookup test covers active Craft, active generic, deleted Craft, absent and wrong-tenant Sessions.
