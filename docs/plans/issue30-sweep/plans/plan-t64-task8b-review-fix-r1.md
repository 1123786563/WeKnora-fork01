# T64 Task 8B Review Fix R1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve all valid findings from the first independent review of T64 Task 8B without changing downstream 8C/8D/8E ownership.

**Architecture:** Preserve Task 8B's existing transaction and service seams. Enforce permanent Variant identity once `published_at` is set, require exactly one assistant placeholder transition for every claim cancellation, align claim keys/indexes across both migration dialects and GORM, and return committed revocation identity with durable reconciliation-pending state when the immediate retry fails.

**Tech Stack:** Go, GORM, SQLite migrations, PostgreSQL/versioned migrations, Go test.

**Spec:** `docs/plans/issue30-sweep/plans/plan-t64-task8-atomic-admission.md` Checkpoint 8B; `docs/adr/0015-agent-security-transactional-admission.md`; `docs/specs/agent-marketplace.md` §§10–11; first review at `/Users/wuyongjun/.codex/worktrees/t64-task8-8b/WeKnora-fork01/.superpowers/sdd/plan-t64-task8-atomic-admission/task-8B-independent-review.md`.

## Global Constraints

- Reconciliation must preserve the exact published Variant attribution, including retired Variants.
- Revocation, audit, claim transition, and assistant placeholder terminalization remain atomic.
- A committed governance revocation remains durably retryable until reconciliation completes.
- Keep SQLite and PostgreSQL/versioned schema contracts equivalent.
- Do not edit Task 8C Run-admission behavior or any Task 8D/8E files.

## Review Focus

- A retired Variant with non-null `published_at` cannot change its Agent, Version, or Release identity — test repository update rejection and SQLite trigger rejection; statically assert the PostgreSQL trigger predicate.
- A missing/mismatched assistant placeholder aborts and rolls back each cancellation transaction — test revocation claim cancellation, owner cancellation, and expiry cleanup.
- Claim identity and message uniqueness are scoped to the specified tenants, with the revocation lookup index present — assert SQLite and PostgreSQL DDL plus GORM key/index metadata.
- Immediate reconciliation failure cannot make a committed revocation look uncommitted — test release and dependency response identity, pending status, and successful later retry.

## Task 1: Close Task 8B review findings F1–F4

**Depends on:** Task 8B implementation `cbdb08e627b3466cc419815e3108fe18bb0b3bfc`; four findings in the independent review.

**Files:**
- Modify: `internal/application/repository/agent_adoption.go`, `internal/application/repository/agent_adoption_test.go`
- Modify: `internal/application/repository/agent_security.go`, `internal/application/repository/agent_chat_turn_claim.go`, and their focused tests
- Modify: `internal/application/service/agent_security.go`, `internal/application/service/agent_security_test.go`, `internal/types/interfaces/agent_security.go`, `internal/types/agent_security_persistence.go`
- Modify: `internal/types/agent_chat_turn_claim.go`
- Modify: `migrations/sqlite/000126_agent_chat_turn_claims.up.sql`, `migrations/versioned/000205_agent_chat_turn_claims.up.sql`
- Modify: `internal/database/migration_sqlite_versioned_schema_test.go` and focused PostgreSQL migration source-contract tests
- Update: Task 8B implementation report and fix-round ledger/evidence.

**Interfaces:**
- Consumes: `AgentSecurityService.RevokeRelease` / `RevokeDependency`, `AgentSecurityRevocationView`, `AgentChatTurnClaimStore`, and the existing durable `run_cancellation_state` fields.
- Produces: service results that preserve committed revocation identity and expose pending reconciliation explicitly; claim migration contract with composite primary key `(id, source_tenant_id)`, unique `(session_tenant_id, assistant_message_id)`, unique `(session_tenant_id, user_message_id)`, and index `(source_tenant_id, state, release_id)`.

- [ ] Add failing tests for F1–F4 before production changes: retired published Variant mutation rejection; zero-row placeholder update rollback on revocation, owner cancellation, and expiry; tenant-scoped schema key/index constraints; and committed revocation identity plus pending response after immediate reconciliation error.
- [ ] Run focused tests to confirm the new assertions fail for the reported causes.
- [ ] Implement the smallest corrections in the files listed above. Treat every `published_at IS NOT NULL` Variant as immutable; require exactly one placeholder row transition before transaction commit; make both migration dialects and GORM metadata express the same tenant-scoped key/index contract; represent immediate reconciliation failure as a committed pending result while keeping the durable worker retry.
- [ ] Run focused checks serially:
  - `go test ./internal/application/repository -run '^(TestAgentChatTurnClaim|TestRunCancellationReconciliation|TestCancelRunsBySecurityPins|TestAgentAdoption.*Variant|TestAgentSecurityStoreAppendReleaseAndAuditRollsBackTogether)' -count=1`
  - `go test ./internal/application/service -run '^(TestResolvePublishedAgentVersion|TestAgentSecurity.*)$' -count=1`
  - `go test ./internal/database -run '^(TestSQLiteMigrationsCreateVersionedSchema|TestTask8ClaimMigrationEmptyDownUpAndPopulatedDownRefusal|TestTask8RunPinsAndPublishedVariantIdentityAreImmutable|TestTask8PostgresMigrationDeclaresTransactionalSecurityGuards)$' -count=1`
  - `go build ./...`
  - `git diff --check` and `git diff --cached --check`
- [ ] Verify all changed paths against the corrected Task 8B Brief; force-add ignored migration files only by exact path if needed; commit the R1 fix and append command outputs, commit SHA, and any environment limitation to the original Task 8B report.

**Acceptance:** Independent re-review marks F1–F4 addressed with no new critical/high/medium defect; a backend validator passes the focused checks against the exact fix HEAD. PostgreSQL runtime remains explicitly unverified if no server is available; source-contract checks do not count as runtime execution.

**Failure handling:** If exact-one-row handling makes legitimate idempotent replay ambiguous, preserve the existing replay contract by reading and validating terminal state within the same transaction; never silently accept a missing or mismatched placeholder. If service response types cannot represent a committed pending result without breaking callers, add a backward-compatible status field to the existing view and update all in-repository callers/tests. Do not weaken the atomicity invariant.

**Review package base:** `cbdb08e627b3466cc419815e3108fe18bb0b3bfc`.

## Self-review

- F1–F4 each map to a named test family and owned implementation seam.
- Both migration dialects and entity metadata are included for the storage contract; ignored migration paths are already tracked from the original Task 8B commit.
- Service return semantics retain the durable retry state and avoid reporting a committed operation as uncommitted.
- No downstream 8C/8D/8E implementation file is in the owned scope.
- Runtime PostgreSQL verification remains a disclosed environment limitation, not inferred from static checks.
