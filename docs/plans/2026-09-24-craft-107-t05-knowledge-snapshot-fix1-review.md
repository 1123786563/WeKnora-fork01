# T05 knowledge snapshot fix 1: independent scoped re-review

Date: 2026-09-24. Scope: the one-KB cardinality Medium finding in `2026-09-24-craft-107-t05-knowledge-snapshot-task1-review.md`, the fix1 plan/report/task-local patch, approved Craft Spec #107/T05, `CONTEXT.md`, ADR-0004/0009, and the reviewed T08 actor Task 3 contract. Read-only review; no OCR or delegation. This does not review T05 Task 2 admission, worker handoff, Publisher, RunView, or full T05.

## Checkpoint and verification

| Item | SHA-256 / result |
| --- | --- |
| Fix1 task-local patch | `c206980503946900f7bb19f7c91a377d64023524afcb86d15195f5c9014b48df` |
| Graph source before, reconstructed by reversing fix1 patch in a temporary directory | `a24824636b4a8a89ff38d55e5f108dfbd8771976c23bcd5d8fd7ac572e6597fa` |
| Graph source after | `eb070c701046d7b873b95d230bdf5e8b76f4004a0529f0aa330cbe53ac5d3c64` |
| Snapshot test before, reconstructed | `68dcacf91f35938e6db717be034ee188a50dbd3beab65c11a7642aad766a0dc4` |
| Snapshot test after | `1e05041efba08cef6dff1f22f5f9911cbb9e8b6d80cb40c286b7028412d453bd` |
| Existing graph test, unchanged | `cdd3d0e2bdfd26bfcf70cbde454b2c0a5878838eb47e9c5e3043dfafb0f4437a` |

The reverse patch exactly reproduces both reported task-entry hashes. `git diff --check` and `gofmt -d` on the two changed files passed without output. I reran the report's anchored snapshot and T08 actor regressions: `go test ./internal/application/service -run '^TestDurableCraftKnowledgeSelectionSnapshotRoundTrip$|^TestDurableCraftKnowledgeSelectionCompatibilityAndGenericSnapshot$|^TestDurableCraftKnowledgeSelectionRejectsMalformedSelection$|^TestDurableRunSnapshotRoundTripKeepsRuntimeFields$|^TestDurableCraftRunSnapshotKeepsManifestOutOfModelMessage$|^TestParseDurableRunSnapshotRejectsUnknownVersion$|^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor$|^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActorAfterSessionDelete$|^TestExecuteDurableCraftRunRestoresActorAcrossWorkerLeaseRecovery$' -count=1` — exit 0, `ok .../internal/application/service 2.086s`.

## Verdict

- **Scoped Spec compliance: PASS.** The prior Medium finding is closed. `canonicalCraftKnowledgeSelection` now rejects any list longer than one with `craft.ErrInvalidInput`. Both the builder and strict parser call that validator, so neither newly built nor crafted persisted two-KB snapshots pass. Existing zero/one selection, exact query, legacy empty Craft selection, generic snapshot omission, and unchanged model-facing prompt are covered by the focused tests.
- **Scoped code quality: PASS.** The correction is narrow, rejects rather than truncates the invalid selection, retains the earlier strict checks, and leaves the T08 actor path untouched. The focused actor regressions pass on the reviewed graph hash. No remaining critical/high/medium finding was identified in this fix1 delta.

## Remaining scope

This verdict is bound to the fix1 patch and file hashes above. It does not establish authenticated `KnowledgeScope` admission, current KB access checks, selection handoff after worker restart, or production Publisher and RunView behavior; those remain later T05 tasks.
