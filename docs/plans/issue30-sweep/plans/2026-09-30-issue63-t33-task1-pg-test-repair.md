# Issue #63 T33 End Adoption PostgreSQL Test Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the T33 EndAdoption contention regression execute against PostgreSQL when configured and prove that the test observes the End backend waiting on the parent row.

**Architecture:** Put the race body in a `postgres` subtest so the existing `openRunTestDB` harness selects PostgreSQL; assert the opened DB dialect; capture End's PostgreSQL backend PID and observe only that backend's lock wait; release the paused CreateVariant callback via cleanup-safe `sync.Once` on every exit path.

**Tech Stack:** Go tagged integration tests, GORM/PostgreSQL, `TRPC_TEST_POSTGRES_DSN`, existing versioned migration harness.

**Spec:** T33 End Adoption snapshot repair plan and report; independent finding E1 in `docs/plans/issue30-sweep/plans/2026-09-30-issue63-t33-task1-end-snapshot-repair-review.md`.

## Global Constraints

- The test must select PostgreSQL only through the repository's supported harness and fail if `db.Name() != "postgres"`.
- Do not count unrelated lock waits as evidence of EndAdoption blocking.
- Release the paused CreateVariant goroutine even if a test assertion fails.
- If `TRPC_TEST_POSTGRES_DSN` is absent, skip with explicit reason; do not claim PostgreSQL execution.

## Review Focus

- A supplied DSN leads to a real PostgreSQL connection and versioned schema, not the SQLite fallback.
- The observed `pg_stat_activity` row is the End connection's PID and waiting query targets the Adoption parent gate.
- Test cleanup unblocks the create callback on timeout or assertion failure and drains both goroutines.

---

### Task 1: Correct and harden the PostgreSQL race harness

**Dependencies:** EndAdoption source repair `773e6328cc57ba2b67a5757edc7428b2d0204bae`; E1 report `2026-09-30-issue63-t33-task1-end-snapshot-repair-review.md`.

**Owner role:** `backend_implementer`. **Validator role:** `backend_validator`. **Independent reviewer:** `reviewer`.

**Files:**
- Modify: `internal/application/repository/agent_adoption_end_concurrency_pg_test.go`
- Modify: `internal/application/repository/agent_adoption_test.go` only if helper reuse requires it.

**Interfaces:**
- Consume existing `openRunTestDB`, `reopenRunDB`, migrations, and `EndAdoption`/`CreateVariant` repository methods.
- Produce a tagged test whose `t.Name()` contains `/postgres`, `db.Name()` is asserted as `postgres`, and lock-wait query is pinned to End's backend PID.

- [ ] **Step 1: Add a failing harness assertion and safe callback cleanup.** Wrap the race body in `t.Run("postgres", ...)`; assert PostgreSQL dialect before database-specific SQL. Replace bare channel close with a `sync.Once` release function registered in `t.Cleanup` so every exit unblocks the insert callback.
- [ ] **Step 2: Scope lock-wait evidence to End's backend.** Pin the End DB handle to one SQL connection; capture `pg_backend_pid()` on that handle; poll `pg_stat_activity` by that PID and assert `wait_event_type='Lock'` plus the gate query predicate before releasing CreateVariant.
- [ ] **Step 3: Run test selection.** Run `go test -tags semantic_integration ./internal/application/repository -run '^TestAgentAdoptionEndWaitsThenSeesCommittedVariant$' -count=1 -v`. Expected: skip only when DSN is unset; with DSN, the postgres subtest opens PostgreSQL, observes the End PID blocked, lets Create commit, then asserts End rejects and final state is active with non-retired Variant.
- [ ] **Step 4: Run focused non-integration regressions and diff check.** Run EndAdoption sequential lifecycle tests, migration up/down/up and migration-version uniqueness, and `git diff --check`. Expected: local sequential contract remains green; tag-specific test reports the exact environment limitation if skipped.
- [ ] **Step 5: Commit and report.** Commit only the integration harness/test changes and report the environment check, dialect assertion, exact outputs, and remaining DSN limitation at `docs/plans/issue30-sweep/plans/2026-09-30-issue63-t33-task1-pg-test-repair-report.md`.

**Failure handling:** If the shared harness cannot isolate a per-test schema or the End backend PID cannot be held stable, stop and request architecture review; do not fall back to a SQLite wait query or unscoped activity count.

## Plan self-review

- Finding coverage: E1's wrong dialect selection, unscoped activity predicate, and callback leak are all addressed.
- Task boundary: this repair changes test scaffolding only; repository behavior remains unchanged.
- Verification: tagged PostgreSQL execution is evidence only when the test actually runs with a configured DSN.
