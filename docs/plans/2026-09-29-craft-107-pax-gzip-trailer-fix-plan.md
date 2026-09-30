# Plan: Add regression for corrupt gzip trailer after tar EOF

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Close the independent review's LOW test gap for F10/Task J by proving `ExtractArchive` rejects a gzip CRC/ISIZE failure discovered only when draining bytes after a valid tar terminator.

**Architecture:** Preserve the existing bounded stream reader and error policy. Add one test-only fixture mutation to an otherwise valid tar.gz: flip a gzip trailer CRC byte after the tar terminators, then assert extraction fails with `ErrInvalidInput`. Do not change runtime behavior unless RED proves the current drain path fails to propagate the trailer error.

**Spec:** `docs/specs/2026-09-23-craft-web-artifact-spec.md`; OCR R2 repair context: `docs/plans/2026-09-28-craft-107-ocr-r2-plan.md` Task J; review: `docs/plans/2026-09-29-craft-107-pax-fix-review.md`; current task evidence: `docs/plans/2026-09-28-craft-107-ocr-r2-pax-report.md`.

## Global Constraints

- No commit, staging, shared stash, Docker, database, or integration-worktree edits by the implementer.
- Own only `internal/modules/craft/archive_test.go` and this task's plan/report/package artifacts in `/Users/wuyongjun/.codex/worktrees/ocr-pax-budget/WeKnora-fork01`.
- Preserve all existing PAX limits, multi-member gzip drain, tar compatibility and fail-closed semantics.
- Do not weaken `ErrInvalidInput` classification or mutate production code unless the new test demonstrates a real behavior failure and the minimal correction is separately reviewed.

## Review Focus

- The fixture must be a valid gzip-compressed tar before the single trailer mutation, with valid tar terminators and a deterministic CRC failure.
- Assert `errors.Is(err, ErrInvalidInput)` and meaningful error context without relying on unstable standard-library wording.
- Demonstrate RED on the pre-change state is not expected if runtime behavior already passes; the test may pass immediately and close only coverage.
- Run focused test, archive test selector, and `git diff --check`; preserve report/hash evidence.

## Task 1 — Corrupt gzip trailer regression

**Dependencies:** none; current implementation and interfaces are frozen at HEAD `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159` plus uncommitted PAX checkpoint. **Role:** backend implementer. **Validator:** backend validator. **Independent reviewer:** parent-dispatched reviewer after implementation.

**Consumes:** current bounded tar stream-drain implementation in `archive.go`; PAX checkpoint source/test hashes recorded in `docs/plans/2026-09-28-craft-107-ocr-r2-pax-report.md`.

**Produces:** a single deterministic test in `archive_test.go`, a task-local incremental review package (including exact before/after test file), and `docs/plans/2026-09-29-craft-107-pax-gzip-trailer-fix-report.md`.

**Steps:**

1. Record HEAD, exact current `archive_test.go` SHA, tracked status, and target file preimage in the task's SDD directory.
2. Add a failing-or-immediately-passing regression using a minimal valid tar.gz with one member. Confirm the tar writer and gzip writer close successfully; flip one bit in the gzip CRC trailer only after both are closed.
3. Assert `ExtractArchive` returns an error wrapping `ErrInvalidInput`. If it currently returns nil, first prove the malformed trailer mutation is validly targeted and then make the smallest production fix below the tar reader; do not alter PAX accounting or limits.
4. Run `go test ./internal/modules/craft -run '^TestArchiveRejectsCorruptGzipTrailerAfterTarEOF$' -count=1`, `go test ./internal/modules/craft -run 'TestArchive' -count=1`, and `git diff --check`.
5. Save exact command outputs, pre/post hashes, incremental patch hash, and limitations in the report/package. No commit.

**Expected result:** new test fails without trailer propagation and passes after the minimal fix; all archive tests pass and no whitespace errors remain.

**Failure handling:** if Go's gzip reader or tar stream behavior makes CRC mutation ambiguous, retain raw fixture/hash and use a deterministic malformed ISIZE/CRC mutation with a direct gzip validation control; do not waive the finding without proof. If unrelated tests/processes contend for the package, stop and report the handle/resource rather than starting another run.
