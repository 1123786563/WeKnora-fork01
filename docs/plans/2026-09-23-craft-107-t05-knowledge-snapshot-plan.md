# T05 Durable Knowledge-Base Selection Plan

> **For Codex:** Execute with SDD RED → GREEN → REFACTOR, exact uncommitted checkpoints, independent Spec/quality Review. No commit.

**Goal:** Freeze the authenticated Craft Run's selected knowledge bases and retrieval query in its immutable durable snapshot, then carry that typed selection to the T05 builder without treating KB IDs as document IDs or trusting later model arguments.

**Sources:** Approved Spec #107/#124 user story 2, T05 worker seam map, reviewed KB-selection/truncation and dispatch-recheck checkpoints, T08 actor principal plan, existing `CraftRunRequest.KnowledgeScope`, `DurableRunSnapshot`, `BuildForKnowledgeBases` and T01 input manifest.

**Global Constraints:** T08 actor Task 2 currently owns `craft_session.go`. T19 sandbox Task 4a released `agent_run_graph.go` without edits after documenting a provider interface blocker; a future Task 4b must not edit graph concurrently with this snapshot Task. Wait for the T08 actor exact checkpoint/review, recheck graph hash, and reserve graph ownership before implementation. Task scope storage owner and durable actor must remain distinct; KB IDs are only selection data until current per-KB ACL is checked. No model tool argument may add KBs. Existing generic Run snapshots and old Craft snapshots must be parsed under a deliberate fail-closed compatibility rule. T01 input manifest and version semantics remain unchanged.

**Review Focus:** stable canonical one-KB selection from the current `knowledge_scope` contract (empty means no selected KB; no silent all-KB fallback), exact admitted query, snapshot idempotency digest, current authority before build/dispatch, replay/worker restart preserving selection, malformed/duplicate/foreign selection denied.

**Task 2 clarification, 2026-09-24:** `CraftKnowledgeSelectionSnapshot.Query` is the authenticated original `CraftRunRequest.Prompt` used for retrieval. The top-level `DurableRunSnapshot.Query` may contain server-composed input/base-Version guidance for the model; it is a different value and must not silently become the KB retrieval query. Remove Task 1's equality assumption between these two fields while keeping strict validation of the typed retrieval query and immutable admission digest. Task 2 backend implementer is granted narrow additional ownership of `internal/application/service/agent_run_graph.go` and `agent_run_snapshot_test.go` solely for this distinction, sequentially after the Task 1 fix1 independent PASS. Both fields remain server-owned and replay-stable; no client/model argument can add KBs or rewrite the retrieval query.

## Task 1 — typed immutable snapshot contract

**Depends on:** T19 Task4a `agent_run_graph.go` file release with unchanged hash and T08 actor Tasks 2–3 review. T08 Task3 takes the shared graph file first to restore the initiating principal; this Task starts only after its checkpoint is integrated/reviewed. **Role:** backend_implementer. **Owned files:** `internal/application/service/agent_run_graph.go`, focused `agent_run_graph_test.go` and new snapshot tests; no `craft_session.go` yet. **Produces:** `CraftKnowledgeSelectionSnapshot{Query, KnowledgeBaseIDs}` in `DurableRunSnapshot`, strict parse/validation and a Craft builder input.

1. RED tests: exact one-KB ID and prompt query round-trip; malformed/duplicate/oversized/null selection fails; generic snapshot unchanged; old Craft snapshot without knowledge selection has an explicit empty-selection compatibility behavior; no model-facing query expansion.
2. GREEN: introduce the optional typed field and validation, extend the Craft snapshot builder without changing generic builder, retain the already admitted input manifest. New snapshot bytes remain stable for same request and feed existing request digest.
3. Focused service snapshot tests, diff check, exact checkpoint and independent review.

## Task 2 — authenticated admission supplies selection

**Depends on:** Task1 reviewed/integrated and T08 actor Task2 reviewed. **Role:** backend_implementer. **Owned files:** `internal/application/service/craft_session.go`, focused Craft session/handler tests. **Consumes:** current request `knowledge_scope` and authenticated actor; **Produces:** durable typed KB selection and query.

1. RED tests: admitted Run freezes one selected KB and original prompt, exact retry reuses it, changed selection conflicts, empty selection remains empty, actor replay cannot swap selection, invalid KB ID is rejected. Current TaskWrite and KB access are checked before material retrieval; no owner/actor authority confusion.
2. GREEN: canonicalize the single KB ID into a typed one-element slice or explicit empty selection, pass into the snapshot builder; never append KB ID to model prompt or reinterpret as document ID. Preserve T01 inputs/base Version and T08 actor wiring.
3. Focused admission/restart tests, exact checkpoint and independent review.

## Task 3 — worker BuildForKnowledgeBases handoff

**Depends on:** Tasks1–2 reviewed, T08 worker actor restoration reviewed and T01 RunView publisher/root availability. **Role:** backend_implementer. **Owned files:** assigned after T08/T19 graph/delegate ownership release. **Consumes:** exact durable snapshot and Run fence; **Produces:** T05 package for the selected KBs only.

Pass server-only selection to `BuildForKnowledgeBases`, bind its complete published package to the same RunView, and recheck current Task/KB/document authority at fresh delegate dispatch. Tests cover replay, revocation, no selection, Run A/B isolation and no model override. Full T05 is not verified until production publisher and OS read boundary pass.

**Failure handling:** If KB access cannot be checked from the authenticated actor or RunView root is not enforceable, keep execution fail closed and report the exact missing interface; do not fall back to shared writer or all-KB search.
