# Issue #63 T33 End Adoption Snapshot Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate the remaining PostgreSQL READ COMMITTED race where EndAdoption can miss a Variant committed while End waits on the Adoption parent gate.

**Architecture:** EndAdoption must acquire the same active-Adoption write gate used by CreateVariant in one statement, then inspect child Variant states in a subsequent statement with a fresh READ COMMITTED snapshot, and only then update the Adoption to ended in the same transaction. Preserve tenant scoping, transition errors, and atomic rollback. Add a PostgreSQL integration regression that pauses Variant creation after its gate, starts EndAdoption while it waits on that parent row, then confirms the committed Variant is visible to the later child check.

**Tech Stack:** Go, GORM, PostgreSQL READ COMMITTED integration test (`TRPC_TEST_POSTGRES_DSN`), existing SQLite repository tests.

**Spec:** T33 plan Task 1; repair review report `docs/plans/issue30-sweep/plans/2026-09-30-issue63-t33-task1-review-repair-review.md` finding R1; ADR-0011 and approved marketplace spec §§8–10.

## Global Constraints

- End and Create use the same durable parent-row gate; service prechecks and in-memory locks are insufficient.
- The child check must be a separate SQL statement issued only after End acquires the parent gate.
- Preserve the invariant that Adoption cannot end while any Variant is not retired.
- PostgreSQL test evidence must use separate connections and READ COMMITTED; SQLite cannot substitute for row-lock/snapshot semantics.
- When `TRPC_TEST_POSTGRES_DSN` is absent, report the environment limit and do not claim PostgreSQL race coverage.

## Review Focus

- CreateVariant gates the parent first, then pauses before child insert; End waits on that same parent gate.
- After Create commits, End's subsequent child query sees the newly committed Variant and returns the lifecycle transition error.
- No final database state has an ended Adoption with a non-retired Variant.
- Direct End with an existing non-retired Variant remains rejected; End after all variants are retired still succeeds.

---

### Task 1: Split EndAdoption parent gate, child read, and final CAS

**Dependencies:** repair implementation `868c8da752494db8d8477292e67a9a1ed09f9b06`, review `2026-09-30-issue63-t33-task1-review-repair-review.md`.

**Owner role:** `backend_implementer`. **Validator role:** `backend_validator`. **Independent reviewer:** `reviewer`.

**Files:**
- Modify: `internal/application/repository/agent_adoption.go`
- Test: `internal/application/repository/agent_adoption_test.go`
- Create only when using the PostgreSQL integration tag: `internal/database/agent_adoption_end_concurrency_pg_test.go`

**Interfaces:**
- Consume existing `AgentAdoptionRepository.EndAdoption` and `CreateVariant` signatures.
- Produce an EndAdoption transaction that conditionally write-gates the tenant-scoped parent in `active`, runs a separate query for non-retired child Variants, then updates that parent to `ended` with actor/time/reason metadata and active-state predicate.
- Preserve existing public repository errors for missing Adoption and invalid lifecycle transition.

- [ ] **Step 1: Add the PostgreSQL race regression.** Under the repository's PostgreSQL integration build convention, create a per-test schema and run the versioned migrations. Use separate DB connections. Install a test-only GORM create callback that pauses Variant insertion after CreateVariant has acquired the parent gate. Start EndAdoption, prove its parent gate is waiting on the held row, then release Variant creation and wait for both calls. Expected: Variant creation commits; EndAdoption returns `ErrAgentAdoptionTransition`; the final parent is active and the child is non-retired. Keep ordinary sequential SQLite guard cases in `agent_adoption_test.go`.
- [ ] **Step 2: Run the regression against the current implementation.** Run the tagged PostgreSQL test with `TRPC_TEST_POSTGRES_DSN` when configured. Expected: before the fix, the old single `UPDATE ... NOT EXISTS` path can incorrectly end after waiting; otherwise document why the test's lock ordering does not reproduce the snapshot case and adjust the synchronization, not the assertion.
- [ ] **Step 3: Refactor EndAdoption to the three-statement transaction.** Begin transaction; conditional no-op update of `(tenant_id,id,state='active')`; if not one row, distinguish not-found vs lifecycle transition; issue a separate child Variant count query for any `state <> 'retired'`; if present return `ErrAgentAdoptionTransition`; finally update the Adoption lifecycle metadata with the active predicate and require one affected row. Any error rolls back the transaction.
- [ ] **Step 4: Verify both winner orders and sequential states.** Add/retain tests that when End gates first, later CreateVariant fails; when CreateVariant gates first and commits, End fails; with all Variants retired, End succeeds; wrong tenant/missing ID errors remain stable. Run the focused repository tests and optional PostgreSQL race test.
- [ ] **Step 5: Run migration and diff checks.** Run the Issue 63 lifecycle SQLite migration up/down/up and migration-version uniqueness tests plus `git diff --check`. No migration should be required for this query-only repair.
- [ ] **Step 6: Commit and report.** Commit only the owned repository/test changes and report PostgreSQL DSN presence, tested isolation level, synchronization evidence, exact command results, diff hash, and limitations at `docs/plans/issue30-sweep/plans/2026-09-30-issue63-t33-task1-end-snapshot-repair-report.md`.

**Failure handling:** If PostgreSQL integration infrastructure cannot prove End is blocked behind Create's parent gate, keep the regression marked incomplete and report the exact missing observable lock seam. Do not interpret a sequential test or SQLite writer lock as PostgreSQL READ COMMITTED evidence.

## Plan self-review

- Finding coverage: directly resolves independent review R1 and preserves the F1 aggregate invariant.
- Interface consistency: no API, schema, or service interface changes; downstream Task 2–4 remain blocked until this checkpoint is reviewed.
- Verification: PostgreSQL is the proof for row-lock/snapshot behavior; SQLite covers the portable sequential transition contract only.
