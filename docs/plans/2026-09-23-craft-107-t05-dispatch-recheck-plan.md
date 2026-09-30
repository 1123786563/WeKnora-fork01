# T05 Dispatch-Time Knowledge Authority Plan

> **For Codex:** Execute with SDD RED → GREEN → REFACTOR, exact uncommitted checkpoint, independent Spec/quality Review. No commit.

**Goal:** Give the final Craft delegate dispatch a narrow, read-only authority check for the exact published Run knowledge record, including current TaskWrite and every recorded document/KB grant.

**Sources:** Approved Spec #107/#124, T05 worker seam map, reviewed KB-selection/truncation and publisher checkpoints, `t05-kb-truncation-fix-review.md`, current `CraftKnowledgeService` and record store.

**Global Constraints:** This task does not wire the worker or publisher and cannot claim T05 completion. Do not rebuild, republish, search, or resolve a model-controlled knowledge ID during the check. Fail closed if records/task/access/search ports are unavailable. Keep immutable record/source identities. `craft_knowledge.go` and focused tests are exclusively owned here; `craft_session.go` belongs to T08 actor Task 2 later, and T01 RunView files belong to another worker.

**Review Focus:** authenticated caller must equal the durable actor scope; TaskWrite now, published state and exact tenant/session/Run match; each recorded source still resolves under current document and owning KB access. Empty package still needs TaskWrite and published state. No check may be satisfied by a saved historical ACL decision alone.

## Task 1 — service revalidation method

**Depends on:** T05 KB selection/truncation fix reviewed PASS. **Role:** backend_implementer. **Owned files:** `internal/application/service/craft_knowledge.go`, `internal/application/service/craft_knowledge_t05_test.go` and a new report/checkpoint. **Consumes:** `craft.Scope`, RunID, immutable record; **Produces:** `RevalidateForDispatch(ctx, scope, runID) error` for a later delegate wiring Task.

1. RED tests: published Run with valid actor/TaskWrite and all current source grants succeeds; revoked TaskWrite/document/KB fails; forged caller or wrong tenant/session/Run fails; prepared/unknown publication state fails; empty published package still checks TaskWrite; records/access/search nil fail closed. Assert no Publisher operation or source search/rebuild runs.
2. GREEN: load the exact accepted record and validate scope/publication; reuse the existing per-source `reauthorizeKnowledgePackage` or a narrow shared helper where possible. The method must not expose source bytes or mutate the record. Keep a distinct service API from `BuildForKnowledgeBases` so dispatch does not republish.
3. Run focused T05/source-guard tests, `git diff --check`, save exact source/test hashes and full-content checkpoint; request independent Review. The subsequent Craft delegate dispatch task cannot start until this method is reviewed and the actor principal path is integrated.

**Failure handling:** If the accepted record lacks enough KB identity for a fresh check, report the precise field/contract gap and stop. Do not infer authority from ref path text alone or waive the check.
