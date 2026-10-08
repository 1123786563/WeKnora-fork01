# Issue 140 Task30 Review Repair R2 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent transaction-error fallback from treating a legacy unknown upload as terminal and releasing its only deletion barrier.

**Architecture:** Use one shared terminal-state predicate for both the normal `ClaimUpload` transaction path and its post-transaction-error fallback. Legacy `failed/interrupted` sources with no durable resource reference stay nonterminal; when the retry transaction fails, return the error without granting the Handler permission to release the lifecycle claim.

**Tech Stack:** Go, GORM, SQLite Career lifecycle persistence.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`, `CONTEXT.md`, ADR-0015–0018; parent plans `docs/superpowers/plans/2026-09-30-issue140-task21-review-repairs.md` Task26 and `docs/superpowers/plans/2026-09-30-issue140-task26-review-repairs-r2.md` Task30; finding `/tmp/issue140-task30-review.md`.

## Global Constraints

- Unknown physical writes remain lifecycle-blocking until the stable object is reconciled or physically removed.
- A failed database transaction is not evidence that a legacy no-ref upload has no physical object.
- Existing SQLite/PostgreSQL migration IDs remain unchanged; no schema migration is expected.
- Local commits only; no push, shared merge, deploy, publication, or GitHub Issue mutation.

## Review Focus

- Legacy `failed/interrupted` + empty-ref same-request retry transaction fails before status transition; fallback must not return terminal and Handler must not resolve lifecycle claim.
- Deletion remains busy after transaction failure and the physical object remains unclaimed/unremoved.
- A subsequent successful retry re-enters the processing path with the same source ID and deterministic object key.
- Ordinary `ready` and truly terminal known-ref/compensated failures retain their existing replay semantics.

---

## Task 32: Keep legacy no-ref retries nonterminal on database errors

**Dependency:** Task30 commit `f3d542427eb9abad76e06e7a5fb81973b36047cf`; finding F2 High in `/tmp/issue140-task30-review.md`.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Owned files:** `internal/career/profile_intake.go`, focused `upload_test.go` / `handler_test.go`, and a test-only DB fault-injection helper if required. Do not change provider or migration files.

**Consumes / produces:** Normal transaction and fallback lookup must share the same predicate: `ready` is terminal; `failed` is terminal except `ResourceRef == "" && ErrorCategory == "interrupted"`, which remains retryable. On failed retry transaction for that legacy case, return the transaction error or typed busy/unknown result with `terminal=false`; never tell Handler to resolve its lifecycle claim.

**Steps:**

- [ ] Add a failing regression with a legacy `failed/interrupted` empty-ref row and durable `source_upload` lifecycle claim; inject a failure in the transition UPDATE inside `ClaimUpload` after initial lookup.
- [ ] Drive the exact request through Handler or the narrowest equivalent public seam; assert it does not return a terminal failed receipt or release the owner claim, and `beginLifecycleDeletion` still returns busy.
- [ ] Remove the injected fault, retry the same request, and assert it reuses the original source ID and deterministic key; verify deletion cannot return `deleted` while the object exists.
- [ ] Run the test RED and record the fallback returning terminal before any change.
- [ ] Extract/use a single terminal predicate for transaction and fallback paths; preserve other error and idempotency behavior.
- [ ] Run focused legacy retry/lifecycle tests, Career package, race subset if touching shared state, and `git diff --check`; append exact evidence to `/tmp/issue140-task30-review-repairs-r2-report.md`.
- [ ] Commit the repair.

**Acceptance:** The fallback can never release the lifecycle claim for a legacy unlocated write; deletion remains blocked through database failure and proceeds only after successful same-request reconciliation and object removal.
