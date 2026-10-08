# Issue 140 Task21 Lifecycle Gate Review Repairs — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the three independently verified Task21 lifecycle-gate findings so retries on every configured storage provider remain discoverable, ambiguous uploads keep deletion blocked until reconciled, and busy duplicates never mutate upload claims.

**Architecture:** Keep the Task21 durable owner token and cross-process guard. Make every configured FileService honor deterministic caller-provided Career object names. Preserve lifecycle ownership across uncertain write/catalog outcomes, and make duplicate guard contention read-only.

**Tech Stack:** Go, GORM, configured object storage FileServices, SQLite/PostgreSQL lifecycle claims.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`; `CONTEXT.md`; ADR-0015 through ADR-0018; parent plan `docs/superpowers/plans/2026-09-30-issue140-career-review-repairs-r2.md`, Task21; findings in `/tmp/issue140-r2-task21-review.md`.

## Global Constraints

- Preserve migration history: #30 SQLite 112–123 / versioned 191–202; #140 SQLite 124–144 / versioned 203–223. Never renumber existing migrations; any further schema migration appends after 144/223.
- Career deletion may return `deleted` only after all admitted external writes are known terminal and all discoverable private objects are removed.
- Duplicate retries must not release another attempt's lifecycle claim or create a claim they do not own.
- Keep caller-selected Career object identity stable across all configured local, S3, MinIO, COS, TOS, OSS and OBS FileService adapters.
- No database transaction may remain open across network/object-storage calls.
- Local commits are authorized; do not push, merge to a shared branch, deploy, publish, or mutate GitHub Issues.

## Review Focus

- A lost object-storage response followed by retry on every configured provider resolves to one stable physical key.
- SaveBytes success followed by catalog-bind or reference-persistence failure cannot release the lifecycle claim until the known object is durably referenced or successfully removed.
- A SaveBytes error with an unknown outcome cannot be treated as proof that no physical object exists.
- A duplicate arriving after guard acquisition but before source-row creation performs no mutation and cannot prevent the owner from processing.
- Same-request replay and deletion remain live after process restart and across Office instances.

---

## Task 26: Reconcile provider writes and keep upload ownership fail-closed

**Dependency:** Task21 commit `d1d85ddda055cc87c1f38b6feeb0942ac70dee7f`; findings T21-1, T21-2, T21-3 in `/tmp/issue140-r2-task21-review.md`.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Owned files:** `internal/application/service/file/{cos,tos,oss,obs}.go` and focused provider tests; `internal/career/{handler.go,upload.go,profile_intake.go}` and focused upload/lifecycle tests. No migration is expected. If code inspection proves a schema change is necessary, stop and report the proposed append-only migration pair before changing files; existing IDs remain fixed.

**Consumes / produces:** Existing `FileService.SaveBytes(ctx, data, tenantID, fileName, temp) (resourceRef, error)`. Career filenames `career_source_<sourceID><ext>` and `career_export_<stable-id>.<ext>` are stable caller identities. Every configured adapter must map these names to one deterministic tenant-scoped object path for retries, while retaining legacy unique naming for unrelated uploads and temporary exports. `CareerOffice.ClaimUpload` remains the mutating owner claim; guard-busy handling must use a read-only lookup or return busy without changing rows. Upload terminal failure may release its lifecycle token only after a durable reference plus completed compensation, or after proving no write occurred; unknown outcomes remain unresolved and deletion stays busy.

**Steps:**

- [ ] Add provider-shaped lost-response tests for COS, TOS, OSS and OBS key construction/adapters; test caller-derived Career names map to the identical object path across retries, including configured bucket/path prefixes, while unrelated and `temp` writes keep legacy behavior.
- [ ] Add a Career upload test that simulates SaveBytes persisting bytes then returning an error; assert the lifecycle claim remains unresolved, deletion stays busy, and same-request recovery reconciles the deterministic object path before terminal cleanup.
- [ ] Add a Career upload test where SaveBytes returns a resource reference but catalog binding fails; assert the reference is durably recorded before cleanup and the lifecycle claim is retained until Delete succeeds, with deletion unable to finalize while cleanup is paused/failing.
- [ ] Add a deterministic interleaving test that pauses owner A after guard acquisition and before source-row insertion, sends identical duplicate B, and asserts B creates no row/token and A proceeds to parsing/success.
- [ ] Run each new test before implementation and record the failing behavior.
- [ ] Implement deterministic Career key mapping in the remaining configured providers, matching the already-supported local/S3/MinIO naming contract and preserving non-Career behavior.
- [ ] Change upload state transitions so physical-write uncertainty cannot clear the durable lifecycle claim; persist returned resource references before later catalog operations, and clear claims only after durable reconciliation or verified compensation.
- [ ] Replace the mutating duplicate fallback with a read-only pending-upload lookup; if no matching pending source exists, return the existing busy/processing response without inserting or updating a row.
- [ ] Run focused provider and Career tests, `go test -count=1 ./internal/career ./internal/application/service/file ./internal/workbench/service/workbench ./internal/container ./internal/database`, targeted Career race tests, and `git diff --check`. Run configured PostgreSQL tests if available; otherwise record the limitation.
- [ ] Commit Task26 and append exact verification evidence to `/tmp/issue140-r2-task21-report.md`.

**Acceptance:** T21-1, T21-2, and T21-3 are all addressed with deterministic regression evidence. No configured storage backend can orphan a Career object due to retry identity; uncertain uploads remain deletion-blocking until reconciled or removed; a busy duplicate performs no mutation.
