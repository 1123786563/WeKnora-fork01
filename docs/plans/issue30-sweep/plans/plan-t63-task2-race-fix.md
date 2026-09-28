# T63 Task 2 Review Fix — serialize adoption end and variant creation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or executing-plans. This is a new numbered SDD repair wave and does not repeat completed Task 2 work.

**Goal:** Fix the accepted HIGH finding that concurrent EndAdoption and CreateVariant can leave a draft variant under an ended adoption.

**Architecture:** Put the shared synchronization boundary in the repository, where both writers run in DB transactions. Their first transactional SQL operation acquires the same tenant/id-scoped parent row write lock, conditional on expected adoption state. Use a guarded no-op UPDATE agent_adoptions SET state = state rather than SELECT FOR UPDATE alone: PostgreSQL serializes on the row; SQLite ignores FOR UPDATE but the first write obtains its writer reservation before any read. Hold the lock through variant counting plus end transition, or through variant insertion. Recheck state inside CreateVariant; the service's earlier read is only a fast path. Map repository state refusal to existing service 409 sentinels for adoption draft creation and upgrade acceptance.

**Tech Stack:** Go, GORM transactions/callbacks, file-backed SQLite production migration DB; optional PostgreSQL integration via TRPC_TEST_POSTGRES_DSN.

**Spec:** Issue #63 snapshot docs/plans/issue30-sweep/issues/issue-63.md; original docs/plans/issue30-sweep/plans/plan-t63.md Task 2 and Review Focus 2; reviewed checkpoint a4ab413a3e40f3f2bee4309cebc9d6c33f7f8ac1; independent HIGH finding in the Task2 review recorded in B6 ledger/report.

## Global Constraints

- Preserve repository interfaces/signatures; no migrations or schema changes.
- Keep tenant ID and adoption ID on every parent guard and child query.
- Repository is the concurrency authority; do not rely only on a service pre-read.
- Missing parent stays ErrAgentAdoptionNotFound; inactive existing parent returns repository ErrAgentAdoptionTransition, mapped to ErrAgentAdoptionStateConflict or ErrAgentUpgradeStateConflict at service callers.
- Do not change separately documented upgrade proposal dual-accept orphan semantics.
- Add failing tests before code; demonstrate RED. Local commits authorized; do not delegate or touch other worktrees.

## Review Focus

1. EndAdoption and repository CreateVariant begin their transaction with the same guarded active-parent write and hold it through check/mutation.
2. Parent lock is tenant-scoped and differentiates missing from inactive without creating a child row.
3. End-first schedule ends adoption and rejects waiting create; create-first schedule inserts variant and makes waiting end fail its active-variant precondition.
4. Previously read active adoption followed by committed end and later repository CreateVariant fails; no child under ended adoption.
5. Both service callers map repository refusal to existing HTTP conflict sentinels.
6. Mandatory tests use file-backed SQLite and separate pool connections. Run analogous optional PostgreSQL evidence if TRPC_TEST_POSTGRES_DSN is set; otherwise report that boundary explicitly.

### Task 1: Close the parent-row race and prove both operation orders

**Dependencies:** T63 Task2 review checkpoint a4ab413a3e40f3f2bee4309cebc9d6c33f7f8ac1; T63 Task1 migration checkpoint 4385f6d6787e2094a46fc0b83f71c9167e62544c.

**Role:** backend_implementer; independent reviewer; backend behavior validation after implementation.

**Worktree:** Existing codex/issue30-t63 worktree only, confirmed at Task2 checkpoint before editing.

**Owned files:** internal/application/repository/agent_marketplace_lifecycle.go, internal/application/repository/agent_adoption.go, internal/application/repository/agent_marketplace_lifecycle_test.go, internal/application/service/agent_adoption.go, internal/application/service/agent_upgrade.go, and focused existing service tests only if required (agent_adoption_test.go, agent_upgrade_test.go). No migrations/router/container changes.

**Report:** .superpowers/sdd/plan-t63-task2-race-fix/task-1-report.md.

**Step 1 — Failing tests first.** Extend the real-migration, file-backed SQLite fixture to allow multiple connections. Register test-local GORM callbacks as deterministic barriers on the guarded no-op parent update, cleanly remove callbacks after each test, and use channels with bounded condition waits (no sleeps). Add:

- End lock wins: pause EndAdoption immediately after its guarded parent write while its transaction holds the DB lock; start CreateVariant, wait until it reaches its guarded write attempt, release End. Require End success, CreateVariant ErrAgentAdoptionTransition, and zero child rows.
- Create lock wins: pause repository CreateVariant after its guarded parent write; start EndAdoption, wait until its guarded write attempt, release Create. Require Create success, End ErrAgentAdoptionEndPrecondition, adoption still active, exactly one non-retired variant.
- Stale service read: read active adoption, end it, then call repository CreateVariant using that previously validated payload. Require ErrAgentAdoptionTransition and no inserted row.

Barriers use channels and bounded condition waits, never sleeps. Use separate SQLite pool connections and _busy_timeout=5000. Test-only GORM callbacks must be registered/removed locally and pause only the exact guarded no-op update. Expected RED against current code. If SQLite returns a serialization error under a schedule, assert neither invalid state commits and explain dialect behavior; stale-read CreateVariant must still incorrectly succeed before the fix, making that test fail.

**Step 2 — Implement the lock-first repository contract.** Add a private helper that is the first SQL operation inside each transaction:

    UPDATE agent_adoptions SET state = state
    WHERE tenant_id = ? AND id = ? AND state = ?

Use parameter binding/GORM expression. On one affected row, proceed. On zero rows, do a tenant-scoped read: missing gives ErrAgentAdoptionNotFound; existing state mismatch gives contextual ErrAgentAdoptionTransition. Avoid using SELECT FOR UPDATE as the only lock.

- CreateVariant: transaction → guarded active parent write/recheck → insert variant → commit.
- EndAdoption: transaction → guarded expectedFrom parent write/recheck → count non-retired variants → precondition error if any → update state and paired timestamps → read back → commit.
- Preserve tenant predicates, unique IDs, existing sentinels, and transaction rollback semantics.
- In AgentAdoptionService.CreateVariant, translate repository ErrAgentAdoptionTransition to ErrAgentAdoptionStateConflict.
- In AgentUpgradeService.AcceptUpgradeProposal, translate the same repository refusal to ErrAgentUpgradeStateConflict, preserving existing 409 behavior.

**Step 3 — Focused verification.** Run:

    go test ./internal/application/repository/ -run 'TestEndAdoption|TestCreateVariant|TestTransitionListingState|TestDeprecateRelease|TestRetiredVariantAgentExists' -count=1
    go test ./internal/application/service/ -run 'TestAgentAdoption|TestAgentUpgrade' -count=1
    go test ./internal/router/ -run 'TestLifecycleEndpointsHappyPathOverRealStack' -count=1
    go build ./...
    git diff --check

If TRPC_TEST_POSTGRES_DSN exists, run equivalent optional PostgreSQL race verification using an isolated test schema and record it. If absent, report PostgreSQL blocked-env; SQLite race schedules are mandatory. A full repository suite that exceeds this bounded scope is not required and must not be counted as passing if interrupted.

**Step 4 — Commit.** Stage only the owned files. Commit message: fix(marketplace): serialize adoption end with variant creation.

**Acceptance:** No schedule permits a variant to commit under an ended adoption; end cannot commit while any non-retired variant exists; stale application pre-reads cannot bypass repository state validation; tenant/missing/conflict semantics and service conflict responses remain; all focused checks pass.

**Failure handling:** If SQLite returns BUSY in a controlled competing write, neither invalid state may commit; inspect exact transaction timing before changing the lock. Do not add generic retries without evidence that busy timeout is insufficient. If deterministic schedules cannot be established with callbacks, stop and report rather than add timing-based tests.

## Interface and Scope Preflight

- The repair consumes T63 Task1 migration columns already present in this worktree and changes only the adoption lifecycle repository/service boundary.
- T64 Task2 must wait until this task is reviewed and integrated; it consumes the same repository state semantics.
- No Marketplace route, migration, unrelated release CAS or UI files are owned.
- Failure blocks T64 repository/security integration and #65, but not isolated #55 work.

## Review Ruling

The reviewer found that EndAdoption's transaction does not serialize with CreateVariant's standalone INSERT, and the service active check can be stale. The architect independently confirmed the common repository boundary and recommended a first-statement parent write lock to cover PostgreSQL row locking and SQLite writer serialization. No schema/API change is needed. If the lock fails to protect either writer, a draft may survive under an ended adoption and violate the all-variants-retired invariant.
