# Craft #107 R5 Task3b Task2 Fix1 Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close F1/F2 in `2026-09-24-craft-107-runview-r5-effect-task3b-task2-review.md` before admitted coordinator sequencing.

**Architecture:** Network attachment map keys are Docker container IDs; every attached ID must inspect to the single deterministic generation container with matching ID/name/labels, or network observation rejects. Immediately before one `ContainerStart`, repeat read-only container/network identity and isolation verification against the caller's expected ID and generation. Only the freshly verified exact ID can be sent. A daemon mutation between check and start is a residual race and remains explicitly documented.

**Tech Stack:** Go Moby engine/provider fakes and focused container tests.

**Spec:** Craft #107 T01/R5 Task3b Task2 review F1/F2.

## Global Constraints

- No effect authority, runtime coordinator, central DI or migration edits. Preserve pure Observe and one-send/no retry. No commit; exact incremental checkpoint. One bounded Docker regression only if fake cannot prove boundary; do not rerun full happy path without reason.

## Review Focus

- Foreign Docker ID with expected-looking endpoint name is rejected from network receipt; only exact ID/name/generation label can pass. Attachment mismatch cannot reach create/start.
- Drifted network attachment, ID, labels, image or security between earlier observation and Start yields zero StartContainer calls; fresh valid observation yields exactly one send.

### Task 1: Exact attachment and pre-start guard

**Depends on:** Task3b Task2 independent review. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/container/craft_runview_docker_engine.go`, `craft_runview_container_provider.go`, focused tests only.

- [ ] RED fakes for foreign attachment ID / matching name and post-observation drift before Start; assert current false acceptance/send.
- [ ] Inspect attached ID and verify exact generation ownership; perform fresh read-only pre-start revalidation before one physical send.
- [ ] Focused/race tests, exact checkpoint/report and independent re-review. Explicitly document check-to-start daemon race.

**Acceptance / failure handling:** No known foreign/drifted resource receives an exact network receipt or a start send; full physical admission remains gated on Task3 and real OpenCode evidence.
