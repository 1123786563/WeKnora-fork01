# T05 Selected-KB Truncation Disclosure Fix Plan

> **For Codex:** SDD RED → GREEN → REFACTOR, exact checkpoint and independent Review. No commit.

**Goal:** Close Medium finding in `t05-kb-selection-review.md`: production `HybridSearch` truncates to requested `MatchCount`, so asking for exactly MaxKnowledgeSources (20) cannot reveal a 21st authorized source; bundle may incorrectly report `Truncated=false`.

**Sources:** Approved Spec #107/#124; `t05-kb-selection-plan.md` and first checkpoint/review; `knowledgebase_search.go:HybridSearch` final cap and `craft.BoundSources` meaning. T05 service path and tests only.

**Global Constraints:** backend_implementer owns only `internal/application/service/craft_knowledge.go`, focused `craft_knowledge_t05_test.go`, report. No craft_session, worker, container, RunView, T19, handler, UI, migration edits or commit/subagents. Other agents share Worktree; preserve all changes. Do not make unbounded retrieval to satisfy truncation.

**Review Focus:** Fetch a bounded sentinel beyond the material cap for each selected KB (at least MaxKnowledgeSources+1). A fake must honor requested MatchCount like production. If sentinel is authorized and otherwise eligible, `BoundSources` drops it and sets Truncated. If search returns a full limit with inaccessible/noisy hits and no proof that eligible hits are exhausted, disclose conservative truncation/partial retrieval instead of claiming complete. Empty and permission-denied cases stay honest; source bytes/citations remain bounded.

## Task 1 — production-faithful sentinel test and fix

1. RED: fake `HybridSearch` clips to `params.MatchCount`, store 21 authorized unique chunks; current typed path reports 20 with Truncated=false. Add multi-KB and unauthorized noise cases, and a below-limit control.
2. GREEN: request a bounded sentinel (21 or documented small overfetch) per selected KB; carry an explicit retrieval-saturated indicator into final bundle Truncated. Never package more than MaxKnowledgeSources, and never expose unauthorized/noisy source metadata. Make test enforce production clipping behavior.
3. Run focused T05 knowledge tests, service package if shared source stable, diff check and exact checkpoint/report; independent re-review. Full T05 still waits durable KB snapshot, worker recheck and RunView isolation.
