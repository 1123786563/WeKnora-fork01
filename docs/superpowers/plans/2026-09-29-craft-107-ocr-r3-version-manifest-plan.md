# Craft Version Draft Head Manifest Fence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Atomically reject a published version whose file manifest differs from the selected draft head manifest.

**Architecture:** Preserve the existing draft-head row lock and immutable revision check. While still inside the version-creation transaction, compare the manifest digest derived from `in.Files` with `expectedHead.ManifestDigest` before inserting the version.

**Tech Stack:** Go, GORM, SQLite repository tests; PostgreSQL runtime is DSN-gated.

**Spec:** `docs/specs/2026-09-23-craft-web-artifact-spec.md`; OCR R3 report `docs/plans/craft-107-ocr-final-3.md`, finding `internal/application/repository/craft_version.go:244-246`.

## Global Constraints

- Draft-head compare, revision lock, and version insert remain in one transaction.
- Compare canonical `craft.ManifestDigest(in.Files)` against the exact locked `expectedHead.ManifestDigest`.
- Preserve existing retry/idempotency semantics for identical version creation.
- Reject mismatched content with `craft.ErrConflict` before any version row is committed.
- No commits; preserve unrelated worktree changes.

## Review Focus

- Exact digest equality succeeds independent of file ordering.
- Changed file bytes/path set fails with `ErrConflict` even when head revision/source identity is unchanged.
- A transaction failure leaves no version row.
- `expectedHead == nil` legacy callers retain current behavior.
- Existing selected-head, stale-revision, and idempotent publish behavior remains intact.

---

### Task 1: Bind published files to the locked selected draft head

**Dependencies:** None. Exact `craft_version.go` and test hashes match the integration baseline; worktree contains the already integrated OCR R2 promotion changes.

**Owner role:** `backend_implementer`.

**Validator role:** `backend_validator`.

**Files:** `internal/application/repository/craft_version.go`, `internal/application/repository/craft_version_test.go`; report/package under this plan's SDD folder.

**Consumes:** The canonical `digest` computed from `in.Files`, selected `expectedHead`, existing revision row and transactional CAS.

**Produces:** An in-transaction `digest == expectedHead.ManifestDigest` precondition, returning wrapped `craft.ErrConflict` on mismatch.

- [ ] Add a regression where head revision/source/digest are valid but `in.Files` differs; assert `ErrConflict` and no version row.
- [ ] Run the new regression and capture RED because the old repository fence accepts mismatched file bytes.
- [ ] Add the digest equality precondition after expected-head structural validation and before version insertion.
- [ ] Run focused repository tests for selected-head publish, stale head rejection, ordering-independent manifest digest, and the new mismatch case.
- [ ] Run `git diff --check`; write final hashes, task-local patch, test evidence, and PostgreSQL DSN limitation.
- [ ] Independent Review gives Spec/code quality verdicts; independent validator checks test results and frozen hashes. No commit.

## Failure Handling

- If the expected head digest uses a different canonicalization than `in.Files`, stop and document both algorithms; do not compare non-equivalent encodings.
- If the test cannot show that no version row was inserted after conflict, strengthen the repository assertion before claiming completion.
