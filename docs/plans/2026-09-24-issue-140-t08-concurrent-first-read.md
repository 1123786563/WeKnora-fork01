# T08 Opportunity concurrent first-read recovery

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Return a request-ID recovery path when a concurrent writer temporarily blocks the first Opportunity receipt read.

**Architecture:** Classification depends on both this invocation's write stage and the possibility of another same-ID writer. A nontransient first-read failure before any local write remains a definite database error. A transient lock/busy first-read failure can coincide with another transaction committing the same request ID, so it enters the same bounded scoped receipt reconciliation as a later uncertain write. The result remains receipt, changed-intent conflict, or typed `outcome_unknown`.

**Tech Stack:** Go, GORM, SQLite concurrency tests.

**Sources:** second fix `8e54ac44c6379077a641aacd563c0f11decbbb0a`, independent reviewer medium finding `opportunity.go:183-191,229-236`; prior plans `docs/plans/2026-09-24-issue-140-t08-backend-review-fixes.md` and `docs/plans/2026-09-24-issue-140-t08-backend-error-classification.md`.

## Root-cause evidence and hypothesis

The concurrent test at `opportunity_test.go:214-225` now accepts a raw `SQLITE_BUSY/LOCKED` from the first receipt SELECT. The local transaction has not written, but another transaction may be committing the same request ID. The second fix keyed recovery solely on `persistenceMayHaveCommitted` for the current invocation, so it classifies this contention as a definite error and maps it to HTTP 500. Hypothesis: treating transient first-read contention as recovery-eligible restores the request-ID contract while retaining raw errors for permanent first-read database failures. Prove the hypothesis by making the existing concurrent test reject raw busy and by keeping the injected permanent-error test.

## Global Constraints

- Change only Career backend code/tests in the existing isolated T08 worktree. No migration, route, Web, guard, or API schema changes.
- Local commits authorized; no push/merge/deploy. Preserve finite 350 ms reconciliation and authenticated owner/tenant scope.
- If the test still yields unrecoverable errors after the narrow classification change, stop and investigate the transaction design instead of adding another symptom patch.

## Review Focus

- No raw SQLite busy/locked reaches the concurrent import caller; the same request ID can be retried to exactly one snapshot.
- Permanent first-read query failure stays a normal database error, and post-write uncertain errors retain recovery.
- Oversized request 413, exact replay, changed-intent conflict, and scope fencing remain intact.

---

### Task 1: Recover transient first-read contention

**Depends:** second T08 backend review fix. **Owner/validator:** backend_implementer / backend_validator. **Files:** `internal/modules/career/opportunity.go`, `internal/modules/career/opportunity_test.go`. **Consumes:** initial receipt query error category. **Produces:** bounded scoped reconciliation for transient contention.

- [ ] RED: Tighten concurrent same-ID test to allow only a receipt or typed `OutcomeUnknownError{RequestID}`, never raw SQLite busy. Run repeatedly. Confirm current failure. Keep injected permanent first-read error test.
- [ ] GREEN: Classify a transient first-read lock/busy error as recovery-eligible; do not broaden permanent errors. Reuse the existing reconciliation path with no additional unbounded retries or duplicate inserts.
- [ ] VERIFY: Focused repeat/race tests, Career/router/database/handler/container suites, `git diff --check`. Save exact code SHA and report, then independent review and validation.

## Shared-file preflight

T08 Web and guard implementation remain paused. This is the only writer to Career code. The public Opportunity wire and migration stay unchanged.
