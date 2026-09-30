# T07 Backend Review Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve all six valid findings in the initial #147 backend Review while preserving source-backed, owner-scoped pending resume proposals.

**Architecture:** Keep T03 confirmed facts and receipts as the sole authority. Give each extracted item a stable key, validate every source-bearing write, and serialize a narrow purpose-specific model DTO. Claim upload request identity before saving bytes; use the same source ID to replay or reconcile processing after interruption, with explicit terminal cleanup and no partial proposal batch.

**Tech Stack:** Go, Gin, GORM, SQLite/PostgreSQL versioned migrations, existing FileService/ResourceCatalog/DocumentReader.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`; `docs/plans/issue-140/issues/issue-147.md`; initial Review `docs/plans/issue-140/reviews/task-7-backend-review-r0.md`. Task worktree `/Users/wuyongjun/.codex/worktrees/issue-140-t07-intake/WeKnora-fork01`, fix BASE `cd81dc222f31a07e5aad4a8f4022f055872f3cd2`.

## Global Constraints

- A system-extracted fact remains a proposal until the user explicitly confirms it; missing/ambiguous fields are never invented.
- Every read/write stays within the authenticated Tenant, owner and single-member Career space; the client cannot certify a `resume_extraction` source.
- Confirmed facts and fact versions survive failed uploads and revision conflicts. Pending/dismissed proposals and raw resume data never enter model inputs.
- No fixed raw resume retention period is invented in this task; the later privacy/deletion task owns that policy.
- Local commits are authorized; no push, merge, deployment, Issue comment or closure.

## Review Focus

- Two different experiences or projects both survive confirmation and remain available as distinct current facts.
- A direct `confirm` cannot write a forged resume source, and model JSON cannot expose an ID number via key or source metadata.
- Identical multipart retry returns the original source and receipt; changed bytes or canonical upload intent under one request ID return conflict without storing another blob.
- Cancellation/crash after durable processing claim is recoverable by source ID or becomes a visible terminal failure with private file cleanup; uncertain completion never triggers duplicate batch writes.
- SQLite/PostgreSQL up/down migrations restore the exact prior schema; tenant/owner isolation holds on recovery and retry.

---

### Task 1: Fact identity, provenance and model privacy

**Depends:** initial backend checkpoint `cd81dc222`; no later task. **Owner/validator:** backend_implementer / backend_validator. **Files:** `internal/modules/career/resume_extract.go`, `resume_extract_test.go`, `office.go`, `office_test.go` or `profile_intake_test.go`, `handler.go`, `handler_test.go`, `model_input.go` and focused tests. **Consumes:** T03 `Proposal`/`Fact`/version identity and source contract. **Produces:** per-item stable fact keys, server-validated source-bearing action, purpose-specific safe model JSON.

- [ ] **Step 1 RED:** Add a test that uploads/extracts two same-category experiences and confirms both through the public Office action; assert both distinct current facts and historical source/evidence survive. Run focused Career tests; expect the new test to fail because both keys are `<category>.details`.
- [ ] **Step 2 GREEN:** Generate stable category + item ID + field keys from explicit source evidence (not array positions); preserve idempotence when extraction order changes. For legitimate direct manual confirm, derive/validate manual provenance server-side. Deny a client-supplied `resume_extraction` source for every write action, including `confirm`, while `confirm_proposal` uses its stored proposal source.
- [ ] **Step 3 RED/GREEN:** Add a handler test for forged direct confirm and a final serialized model-input test with identity-like numbers in fact key, value, source label and reference ID. Serialize only purpose-required fields with safe provenance or omit source metadata from the model DTO; ensure the complete resulting JSON has no irrelevant identity number. Keep confirmed relevant facts.
- [ ] **Step 4 VERIFY:** `go test -count=1 ./internal/modules/career/... ./internal/handler/...`; inspect `git diff --check`. A failure blocks Task 2 and Web contract freeze.

### Task 2: Durable request replay and interrupted upload reconciliation

**Depends:** Task 1. **Owner/validator:** backend_implementer / backend_validator. **Files:** `internal/modules/career/profile_intake.go`, `upload.go`, `handler.go`, their tests, and if needed a narrow Career startup/recovery call in `internal/container/container.go`; `migrations/sqlite/000114_career_source_revisions.{up,down}.sql`, `migrations/versioned/000193_career_source_revisions.{up,down}.sql`, `internal/database/career_migration_test.go`. **Consumes:** authenticated owner scope, existing FileService/ResourceCatalog, T03 receipt lookup. **Produces:** request ID scoped to owner, stable source ID, replay/reconciliation semantics and reversible migration.

- [ ] **Step 1 RED:** Add an HTTP-level test: exact same `requestId`/bytes/intent repeated returns the same source ID and receipt with no new blob/proposal; changed bytes or intent under that ID returns 409 before storage. Add concurrent same-request test that only one parse completes. Run focused tests and observe failure.
- [ ] **Step 2 GREEN:** Require nonempty request ID; compute/validate digest before storage; atomically claim/find `(tenant_id,user_id,request_id)` with unique index and persisted canonical intent/digest/expected revision. Ready replay returns stored source and batch receipt. A processing replay uses the same source ID and returns processing or safely claims a stale processing lease; failed replay returns its original terminal state. Use a deterministic internal batch request ID based on source ID, separate from the public request ID, so the T03 receipt namespace does not collide. No second file save on ready replay.
- [ ] **Step 3 RED/GREEN:** Test interruption after processing claim and after blob-ref persistence, cancellation during parse, unknown result after batch commit, and cleanup failure. Reconcile using source ID and scoped durable metadata: if ready/receipt exists, return it; if processing with private ref, re-read and verify digest before parsing; if no ref, accept the same retry bytes under the same source ID. Known terminal parse failure clears or marks cleanup-pending private ref, with bounded detached cleanup. Unknown DB outcome must query source/receipt before any failure marking or file release. Expose a bounded stale-processing reconciliation path that does not leave processing indefinitely, and demonstrate it in a restart/retry test. Treat the FileService physical-write-before-catalog-registration window explicitly; either close it with a recoverable locator or document the infrastructure residual and seek reviewer ruling, never assert it is solved by normal request retry.
- [ ] **Step 4 MIGRATION:** Add request/lease/recovery fields and unique constraint to SQLite 000114 and PostgreSQL 000193; update GORM model. Make both down scripts remove `career_proposals.evidence` as well as source table/indexes; test SQLite up/down/up and restart, and isolated PostgreSQL apply/down if the existing test harness supports it. Do not write shared development DB.
- [ ] **Step 5 VERIFY:** `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...`; `git diff --check`. Record exact commands, output, HEAD, limitations and remaining physical-storage crash window in the task report. Commit only assigned files. Failure handling: keep T07 running and do not integrate or dispatch Web until reviewer and backend validator approve the exact fix checkpoint.

## Shared-file and interface preflight

Both tasks edit Career `handler.go` and `profile_intake.go`; execute serially in the same isolated worktree. No Web/TypeScript file is owned by this fix. Backend wire `GET /career/sources` and multipart `POST /career/sources/upload` may evolve only with explicit report of response/status changes before frontend dispatch. Migration numbers 000114/000193 are reserved by T07 and follow T04 000113/000192. T08 shares Career migrations/contract and waits for T07 integration. Review Package is actual `cd81dc222..fix_HEAD`; independent reviewer must rule separately on Spec compliance and code quality, and validator must bind tests to the same HEAD. No empty BASE..HEAD or stale test evidence is accepted.
