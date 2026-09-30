# Craft #107 R4 Task 2b Task 2 Fix 1 Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close independent Task 2 review findings F1–F3 so a sealed terminal capture can be replayed and advanced from verified immutable object bytes.

**Architecture:** Keep the existing tenant/workspace/Run receipt state machine. Replace global pending-list lookup in the seal path with a scoped receipt-and-files load. Direct sealed retries use the same stored refs. Verify every sealed object ref against its recorded byte count and SHA-256 before `DraftHeadStore.Advance`; missing or corrupt data remains unresolved and fenced.

**Tech Stack:** Go/Gorm, existing Craft file store, SQLite and PostgreSQL focused migration/schema.

**Spec:** Approved Craft Spec #107; `2026-09-24-craft-107-runview-r4-task2b-plan.md`; independent review `2026-09-24-craft-107-runview-r4-task2b-task2-review.md`.

## Global Constraints

- No default Version publication, runtime wiring or unrelated schema change.
- Sealed retry must use stored refs, never mutable source bytes. Scope all loads and object reads to receipt tenant and exact Workspace/Run; validate metadata and file set before Advance.
- No commit, push or shared stash. Record exact task-local pre/post hashes and patch, including any ignored files; do not attribute concurrent edits in shared lifecycle tests.

## Review Focus

- F1: direct sealed retry after crash advances the same manifest; changed source bytes do not alter it.
- F2: more than 500 unrelated unresolved receipts cannot hide this just-sealed receipt.
- F3: missing, corrupt, wrong-size or wrong-tenant stored object ref prevents Advance and leaves admission fenced.

### Task 1: Scoped sealed replay and object verification

**Depends on:** R4 Task 2 implementation checkpoint and independent review. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/application/repository/craft_run_capture.go`, `internal/application/service/craft_run_capture.go`, their focused `_test.go` files only. The production lifecycle source and migration files remain untouched.

**Consumes / produces:** Existing receipt, sealed file rows and tenant-scoped object refs; produces identical verified receipt for direct replay, bounded scoped seal result, and fail-closed Advance authorization.

- [ ] Capture exact preimage. RED real SQLite service/repository tests for direct `CaptureTerminal` retry after seal, 501 older unresolved receipts, missing/corrupt/wrong-size object and cross-tenant ref. Assert unchanged draft and blocked admission on failure.
- [ ] Add one scoped receipt loader validating sealed metadata/file rows. Call it from existing sealed `EnsurePending` and post-seal return; retain global `RecoverPending` only for recovery worker. Verify sealed object bytes at storage boundary before Advance without rereading live Run source. Ensure upload path either has read-back integrity or relies on a proved content-addressed save contract; test the chosen contract.
- [ ] Run focused repository/service tests and race selectors; verify current migrations still pass as applicable. Save task-local checkpoint JSON, incremental patch and report with commands/results.
- [ ] Independent reviewer checks F1–F3 closure, separate Spec/quality verdict and current hashes. If any finding remains, plan another bounded fix; Task 3 stays gated.

**Acceptance / failure handling:** Only exact verified sealed refs advance the draft. Any unavailable/corrupt object preserves the unresolved receipt; no silent empty manifest or false `ErrNotFound` from global backlog.
