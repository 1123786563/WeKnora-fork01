# T05 durable knowledge snapshot Task 1: independent scoped review

Date: 2026-09-24. Scope: Task 1 of `2026-09-23-craft-107-t05-knowledge-snapshot-plan.md`, its report and exact task-local patch, approved Craft Spec #107 user story 2, `CONTEXT.md`, ADR-0004/0009, and the reviewed T08 actor Task 3 contract. Read-only source review; no OCR or delegation. This review covers only `agent_run_graph.go` and new `agent_run_snapshot_test.go` changes in the task-local patch. It does not approve Task 2 admission, worker handoff, Publisher, RunView, or full T05.

## Checkpoint and verification

| Item | SHA-256 / result |
| --- | --- |
| Task-local patch | `063fccf90976be9c3da77485d9e0551af0eeb3ad418564f025dd08607b017190` |
| Graph source before, reconstructed by reversing the patch in a temporary directory | `5bc1a7a7ddc882ee5b12dd068fbd96fa4c32c78ba0ef5c106288d413de90e4d0` |
| Graph source after | `a24824636b4a8a89ff38d55e5f108dfbd8771976c23bcd5d8fd7ac572e6597fa` |
| Existing graph test, unchanged | `cdd3d0e2bdfd26bfcf70cbde454b2c0a5878838eb47e9c5e3043dfafb0f4437a` |
| New snapshot test after | `68dcacf91f35938e6db717be034ee188a50dbd3beab65c11a7642aad766a0dc4` |

The reverse patch also removed the new test, confirming its complete addition. `git diff --check -- internal/application/service/agent_run_graph.go` passed. The report's combined anchored snapshot and actor regression command passed locally (`go test ./internal/application/service -run '^(TestDurableCraftKnowledgeSelectionSnapshotRoundTrip|TestDurableCraftKnowledgeSelectionCompatibilityAndGenericSnapshot|TestDurableCraftKnowledgeSelectionRejectsMalformedSelection|TestDurableRunSnapshotRoundTripKeepsRuntimeFields|TestDurableCraftRunSnapshotKeepsManifestOutOfModelMessage|TestParseDurableRunSnapshotRejectsUnknownVersion|TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor|TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActorAfterSessionDelete|TestExecuteDurableCraftRunRestoresActorAcrossWorkerLeaseRecovery)$' -count=1`, exit 0). No full service suite was run for this scoped review.

## Verdict

- **Scoped Spec compliance: FAIL.** The snapshot freezes the exact query, encodes explicit empty selection for new and legacy Craft snapshots, keeps generic builder JSON unchanged, rejects malformed/null/duplicate/noncanonical/query-mismatched selections, and leaves the model-facing user message unchanged. However, the new builder and parser accept multiple KB IDs despite the Task 1 brief's current one-KB `knowledge_scope` contract.
- **Scoped code quality: FAIL on the contract boundary.** The focused tests pass and the patch preserves the reviewed actor path, but the tests affirm the overbroad accepted selection. There is no evidence here that admission or worker execution uses this field yet.

## Finding

### Medium — typed snapshot accepts a broader KB scope than the current Craft Run request

**Evidence / affected symbols:** The approved Task 1 plan's Review Focus requires a canonical **one-KB** selection from `CraftRunRequest.KnowledgeScope`, which is a single string (`craft_session.go:731-736`). `canonicalCraftKnowledgeSelection` in `agent_run_graph.go:161-176` delegates nonempty lists to `canonicalCraftKnowledgeBaseIDs`, whose limit is 20, and `validateCraftKnowledgeSelection` accepts that canonical multi-ID list on replay. `agent_run_snapshot_test.go:37-46` explicitly expects two IDs to build and parse successfully.

**Impact:** A durable Craft snapshot can represent two or more knowledge bases even though the current admission contract can authorize at most one selected `knowledge_scope`. If later admission or worker wiring trusts this typed shape, the persisted selection can exceed the member's admitted scope. The data-only Task 1 patch does not itself show a live unauthorized retrieval.

**Smallest defensible correction:** Bound the Task 1 snapshot selection to zero or one ID in both builder and parser, while retaining canonical ID validation and explicit empty behavior. Replace the two-ID success test with a two-ID rejection test; test stable bytes for a one-ID selection. A future multi-KB request contract would need its own admission and authorization decision before widening this durable shape.

## Positive evidence and limits

`BuildDurableCraftRunSnapshotWithKnowledgeSelection` copies the input manifest and selection IDs; the exact prompt is stored as the selection query and must match the top-level query within the 64 KiB Craft prompt bound. Parsing distinguishes an absent field on marked legacy Craft snapshots from explicit null and gives the former an empty list. Generic builder output omits the Craft field. `durableUserMessage` still constructs content from `snapshot.Query` only. The snapshot patch contains no actor-path change, and the focused T08 actor regressions pass on the reviewed graph hash. These are Task 1 observations only, not admission, current KB ACL, replay-to-worker, Publisher, or RunView proof.
