# Craft #107 R5 Effect Task3a Fix 1 Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close R5E-3A-1/2 so a read-only observation or DB-only marker failure cannot consume a one-shot provider claim before any send.

**Architecture:** Perform pure container/state observation and exact identity checks before `BeginEffect`; claim immediately before `CreateGenerationContainer` or `StartGenerationContainer`. Persist/validate the legacy `BeginSessionCreate` DB marker before the OpenCode claim; claim immediately before `CreateOpenCodeSession`. A preclaim error is recoverable without an effect intent. An actual send ambiguity remains durable unknown and never resends.

**Tech Stack:** Go admitted coordinator, fake split provider and effect authority.

**Spec:** Approved Craft #107 T01/R5; `2026-09-24-craft-107-runview-r5-effect-task3a-plan.md`; independent `2026-09-24-craft-107-runview-r5-effect-task3a-task1-review.md`.

## Global Constraints

- Only coordinator source/focused tests; no current combined provider, engine, DI or repository authority edits. Production remains default-off.
- Pure observation must have no hidden network/container/session/filesystem mutation. If provider cannot prove that contract, fail closed at provider interface; do not move a mutating call before its claim.
- Keep exact Task fence/digest, generation and receipt checks, unknown/no-resend after a real attempted send. No commit/push/stash; exact incremental checkpoint.

## Review Focus

- Read-only ObserveContainer/State error or identity mismatch before claim inserts no pending/unknown effect; exact replay can later claim once and send once.
- BeginSessionCreate DB marker failure before OpenCode claim inserts no OpenCode effect; exact replay after marker recovery sends once.
- Post-send response loss/cancellation still parks unknown; duplicate replay never sends again.

### Task 1: Preclaim reads and marker sequencing

**Depends on:** Task3a checkpoint and independent FAIL review. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/container/craft_runview_runtime.go`, `craft_runview_admitted_runtime_test.go` only.

- [ ] Capture exact preimage. RED stateful-authority fake tests for transient observation failure, wrong identity and marker failure, asserting no claim and exact successful retry; retain post-send unknown test.
- [ ] Move only pure reads/marker before relevant one-shot claim. Keep claims adjacent to actual mutation and FinishEffect exact receipt/unknown handling.
- [ ] Run focused/race/container compile, exact checkpoint/report/patch, independent re-review.

**Acceptance / failure handling:** Recoverable pre-send failures cannot strand a Run; actual ambiguous sends remain unresolved. Real provider split/network authority/DI remain later Tasks.
