# Craft #107 T01 R5 Frozen Identity Fix 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the independently reviewed gap where an old Craft Task/Fence digest is not compared with the current durable Run before delegation/material side effects.

**Architecture:** The already persisted canonical snapshot digest is an independent admission identity carried by Task/Fence. Under `CraftStore.PrepareTask`'s Run transition lock, compare Task digest/version to durable row and recompute the latter from final snapshot bytes before writing a delegation. Repeat this exact identity check at the material resolver's pre-allocation boundary; Task2 effect claims later make the check durable through external calls. On admission replay, reject a corrupt stored digest before returning the Run.

**Tech Stack:** Go/Gorm, focused SQLite/PG repository and assembly tests.

**Spec:** `docs/plans/2026-09-24-craft-107-runview-r5-fence-task1-review.md`; `docs/plans/2026-09-24-craft-107-runview-r5-fence-plan.md`; approved Craft #107 T01/ADR-0008.

## Global Constraints

- Compare **original Task/Fence** digest to durable Run digest; deriving both sides from a later `Get` is invalid. Same Workspace ID does not imply same query/model/inputs/knowledge/seed.
- `PrepareTask` must reject before any delegation row; material resolver rejects before coordinator/Docker/OpenCode. This fix does not claim to close the later lease TOCTOU, which requires the already planned Task2/3 per-effect intent.
- Legacy/missing/unknown-version or corrupt digest fail closed. No commit, route enablement or unrelated file edit. Exact checkpoint/independent review.

## Review Focus

- Same Workspace with valid changed snapshot **and matching changed row digest** while Task carries old digest rejects at both boundaries; zero coordinator/engine/session calls.
- Admission replay of corrupt digest fails at replay, not only later ClaimDriver.
- Valid frozen Task remains accepted and existing non-Craft Run behavior is unchanged.

---

### Task 1: Lock and compare frozen identity at preparation and materialization

**Depends on:** R5 fence Task1 independent review FAIL Medium. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/application/repository/craft_workspace.go`, `internal/application/repository/agent_run.go`, focused repository tests; `internal/container/container.go`, `internal/container/craft_runview_assembly_test.go` for the narrow resolver guard/tests. No runtime source or T19 files.

- [ ] Capture exact preimages. RED: replace durable Run snapshot with a different valid same-Workspace snapshot and recompute/persist its matching digest; old Task/Fence reaches PrepareTask or ResolveMaterial today. Assert no delegation row and zero coordinator/Docker/OpenCode after fix. RED replay with corrupt digest, plus missing/deleted/unknown Run zero-call cases.
- [ ] Validate the Task/Fence digest/version against the locked Run row and recomputed canonical bytes in `PrepareTask`. Apply the same independent identity at resolver entry before any side effect. Admission replay recomputes and verifies stored identity. Preserve existing positive path.
- [ ] Run focused repo/assembly tests, migration-aware SQLite/PG where available, gofmt/diff, exact checkpoint/patch/report and independent review.

**Acceptance / failure handling:** Medium identity gap closes; the separate per-effect claim Task2 remains mandatory for lease/revocation race. If the Task lacks original digest on any recovery path, reject it rather than synthesizing one from current row.
