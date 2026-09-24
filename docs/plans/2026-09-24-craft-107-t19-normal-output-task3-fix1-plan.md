# Craft #107 T19 Normal Output Task3 Fix1 Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close T3-1 Medium in `2026-09-24-craft-107-t19-normal-output-task3-review.md`: a provider start error cannot coexist with `OutputComplete=true` or nonpartial output.

**Architecture:** Process/transport/output are separate typed states, but `OutputComplete` requires absence of start/stream/seal error and a positive sealed durable output. Calculate final completeness after error classification or include `startErr == nil` in the predicate; on any start error preserve exact unknown receipt and mark output partial/unverified. Keep one-send and replay unchanged.

**Tech Stack:** Go normal Docker service and fake-provider tests.

**Spec:** Approved Craft #107 T19; normal output Task3 independent review T3-1.

## Global Constraints

- Only service source and focused test. No provider, store, coordinator, production route or migration edits. No Docker rerun needed for fake-provider error branch unless review finds physical dependency. No commit; exact incremental checkpoint.

## Review Focus

- A fake provider returning nonnil `startErr` with otherwise complete-looking terminal/output evidence yields unknown + `OutputComplete=false` + partial output; exact receipt remains and no reattach on replay. Success path unchanged.

### Task 1: Error-first output classification

**Depends on:** Task3 independent review. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/application/service/craft_docker_normal_exec.go` and focused test only.

- [ ] RED fake-provider regression for nonnil startErr with complete-looking evidence and replay.
- [ ] Apply minimal error-aware completeness/partial predicate; keep start permission consumption and durable cursor semantics.
- [ ] Focused SQLite/isolated PG/race checks as affected, exact checkpoint/report and independent re-review.

**Acceptance / failure handling:** Unknown start never reports complete output; production routing remains gated.
