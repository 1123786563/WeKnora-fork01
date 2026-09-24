# Craft #107 T19 Normal Input Task2b Task1 Fix 1 Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close independent review T19-NI-1 and T19-NI-2 before the normal coordinator can stage and claim a physical Docker operation.

**Architecture:** Check normal-stage existence inside each legacy bind/claim transaction after the same Run transition lock used by Stage, before any journal mutation. Bound the canonical request bytes before encryption and after decryption, with a matching persisted ciphertext constraint. Keep exact input/full receipt and existing fail-closed AES behavior.

**Tech Stack:** Go/Gorm, SQLite125/PostgreSQL204 schema and deterministic transaction tests.

**Spec:** Approved Craft #107 T19; `2026-09-24-craft-107-t19-normal-output-task2b-plan.md`; `2026-09-24-craft-107-t19-normal-output-task2b-task1-review.md`.

## Global Constraints

- No coordinator, physical provider, output-store or routing edits. No commit/push/shared stash.
- Stage and legacy Bind/Claim share the exact Run lock; pre-lock lookups cannot authorize mutation. Existing complete-receipt normal API remains unchanged except required compatible validation.
- Reject oversized canonical request before encryption/allocation and on read; never log raw stdin or encrypted payload. Migration constraint must match implementation and be tested in isolated PG and SQLite.

## Review Focus

- T19-NI-1: deterministic Stage-vs-legacy Bind and Claim interleavings cannot leave staged input with three-field authority; transaction-first order behaves consistently.
- T19-NI-2: oversized non-stdin command/env fails before encryption/insert; exact maximum round-trips, corrupt/oversized stored ciphertext fails closed.

### Task 1: Locked legacy exclusion and aggregate stage bound

**Depends on:** Normal input Task2b Task1 checkpoint and independent FAIL review. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/application/repository/craft_docker_send_claim.go`, `craft_docker_normal_input.go`, their focused tests, SQLite125/PG204 up migration files only if DB length constraints change. No coordinator/service files.

**Consumes / produces:** Existing stage and journal Run lock; produces serializable exclusion of legacy authority and bounded encrypted stage bytes.

- [ ] Capture exact preimage. RED controlled transaction tests for Stage committing between old preflight and Bind/Claim; oversized command/env and exact boundary, including malicious stored payload.
- [ ] Move stage-existence checks after lock into each transaction. Define a concrete total canonical-byte cap and compatible ciphertext cap (including AES framing/tag overhead); reject before encrypt and on read. Add DB constraints if feasible across both engines.
- [ ] Focused SQLite/isolated PG, migration up/down/up and race checks; exact incremental checkpoint/report/patch. Independent reviewer checks both findings and no scope drift.

**Acceptance / failure handling:** No legacy three-field path can coexist with a normal stage, and no oversized stage can persist or be read. Coordinator Task2 stays gated until independent PASS.
