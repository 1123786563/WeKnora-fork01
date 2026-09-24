# T05 knowledge snapshot one-KB cardinality fix 1 report

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit)
- Only the authorized graph source and focused snapshot test file were changed. T08 actor production behavior and tests were preserved.
- Task-local patch: `docs/plans/2026-09-24-craft-107-t05-knowledge-snapshot-fix1-task-local.patch`, SHA-256 `c206980503946900f7bb19f7c91a377d64023524afcb86d15195f5c9014b48df`.
- `gofmt -d` returned no diff; `git diff --check` passed for the owned files.

## Change

The current `CraftRunRequest.KnowledgeScope` contract is zero-or-one KB. `canonicalCraftKnowledgeSelection` now rejects more than one selected KB with `craft.ErrInvalidInput`. The builder invokes this validator before marshaling, and the strict parser uses the same validation path before returning a persisted selection. It does not truncate or reorder a two-KB request into an apparently authorized one-KB request.

Snapshot tests retain acceptance of one selected KB and explicit empty selection; the one-KB snapshot remains stable across identical builds, the empty/legacy Craft behavior and generic snapshot JSON stay unchanged, and the original prompt remains the model-facing message. Two IDs are now rejected both when building and when parsing a crafted stored snapshot.

## RED → GREEN evidence

RED before the cardinality change:

```text
go test ./internal/application/service -run '^TestDurableCraftKnowledgeSelectionSnapshotRoundTrip$|^TestDurableCraftKnowledgeSelectionCompatibilityAndGenericSnapshot$|^TestDurableCraftKnowledgeSelectionRejectsMalformedSelection$' -count=1
```

Failed as expected: parser subtest `two_KBs_exceed_current_selection_contract` received no error; builder test expected `craft.ErrInvalidInput` but got nil.

GREEN focused snapshot plus adjacent snapshot/actor regressions:

```text
go test ./internal/application/service -run '^TestDurableCraftKnowledgeSelectionSnapshotRoundTrip$|^TestDurableCraftKnowledgeSelectionCompatibilityAndGenericSnapshot$|^TestDurableCraftKnowledgeSelectionRejectsMalformedSelection$|^TestDurableRunSnapshotRoundTripKeepsRuntimeFields$|^TestDurableCraftRunSnapshotKeepsManifestOutOfModelMessage$|^TestParseDurableRunSnapshotRejectsUnknownVersion$|^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor$|^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActorAfterSessionDelete$|^TestExecuteDurableCraftRunRestoresActorAcrossWorkerLeaseRecovery$' -count=1
```

Passed: `ok github.com/Tencent/WeKnora/internal/application/service 2.231s`. This includes explicit empty and one-KB success, malformed/null/duplicate/oversized rejection, both T08 actorless active/deleted Session tests, and worker actor restoration.

## Owned-file content hashes

SHA-256 at task entry and this checkpoint:

| File | Before | After |
|---|---|---|
| `internal/application/service/agent_run_graph.go` | `a24824636b4a8a89ff38d55e5f108dfbd8771976c23bcd5d8fd7ac572e6597fa` | `eb070c701046d7b873b95d230bdf5e8b76f4004a0529f0aa330cbe53ac5d3c64` |
| `internal/application/service/agent_run_snapshot_test.go` | `68dcacf91f35938e6db717be034ee188a50dbd3beab65c11a7642aad766a0dc4` | `1e05041efba08cef6dff1f22f5f9911cbb9e8b6d80cb40c286b7028412d453bd` |

## Remaining gates

Independent re-review is pending. Task2 authenticated Craft admission wiring, worker BuildForKnowledgeBases handoff, current access rechecks, production Publisher, and per-Run read isolation are still outside this fix and remain T05 gates.
