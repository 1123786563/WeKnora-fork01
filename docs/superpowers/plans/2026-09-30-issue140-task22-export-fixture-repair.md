# Task22 Export Fixture Repair

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this task.

**Goal:** Update Career archive tests to construct submitted progress through the confirmed submission domain operation after Task22 correctly prevents generic submitted events.

**Architecture:** Preserve the generic progress guard and production behavior. Change only export fixtures that still create a submitted event via `AppendProgress`; `RecordSubmission` now atomically creates the linked submitted event and submission record.

**Tech Stack:** Go, GORM, SQLite tests.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`, ADR-0015–0018, Task22 plan, `/tmp/issue140-r2-task4-review.md`.

## Global Constraints
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue140-task22-submission-projection/WeKnora-fork01`; BASE checkpoint `a66cbf155f566dd55b2474cf8f805cf44e4ab7b5`.
- Keep the product restriction that generic `AppendProgress` rejects submitted/resubmitted events.
- Only adapt test fixtures and assertions in Career export tests if necessary; no unrelated test weakening.
- Local commit authorized; no push, merge, deploy, or GitHub mutation.

## Review Focus
- Every export fixture gets exactly one real `RecordSubmission`-linked event.
- Existing archive event/submission payload assertions remain meaningful.
- Full Career package suite passes, not only the changed tests.

## Task 1: Align export fixtures with submission projection
**Dependency:** Task22 initial implementation.
**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.
**Files:** `internal/career/career_export_test.go` only, unless a narrowly adjacent assertion requires correction.
**Consumes / produces:** `RecordSubmission` produces both submission and progress event transactionally.
**Steps:**
- Add/adjust fixture expectations so archive tests first create the actual submission and obtain its linked submitted event without calling generic `AppendProgress` for submitted.
- Run focused export tests and confirm they pass.
- Run `go test -count=1 ./internal/career` and `git diff --check`.
- Commit only owned test changes and report exact BASE/HEAD.
**Acceptance:** Career export tests pass with the new domain contract and no production guard is relaxed.
