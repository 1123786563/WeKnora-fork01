# Craft #107 R5 Task3b Task1 Fix1 Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close R5-3B-1 in `2026-09-24-craft-107-runview-r5-effect-task3b-task1-review.md` without weakening old durable effect replay.

**Architecture:** `BeginEffectWithDigest` only admits the two new request-bound kinds (`docker_network_create`, `docker_probe`). Old `docker_create`, `docker_start`, `opencode_create` continue through `BeginEffect` with empty request digest; a mixed API call fails before inserting an intent. Preserve migration and existing rows.

**Tech Stack:** Go Craft effect repository and focused SQLite tests.

**Spec:** Craft #107 T01 R5 Task3b Task1 and independent Medium R5-3B-1 review.

## Global Constraints

- No migrations, provider, coordinator or DI edits. Preserve one-shot/fence/unknown/receipt behavior for both APIs. Exact incremental checkpoint, no commit.

## Review Focus

- Cross-API old-kind WithDigest attempt is rejected without row; old BeginEffect first claim/replay still works. New-kind BeginEffect requires digest and rejects drift.

### Task 1: Segregate old and new claim APIs

**Depends on:** Task3b Task1 independent review. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/application/repository/craft_run_view_effect.go`, focused effect tests only.

- [ ] RED cross-API test showing old-kind WithDigest creates a row then old BeginEffect replay conflicts.
- [ ] Reject old kinds in WithDigest before transaction/insert; leave legacy method behavior unchanged.
- [ ] Focused/race tests, exact checkpoint/report and independent re-review. Failure keeps Task2 gated.

**Acceptance / failure handling:** Old intents remain replayable through the legacy API; new intents require request-bound digest.
