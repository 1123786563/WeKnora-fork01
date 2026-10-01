# T33 Task 2 Repair Round 1 — Adoption/CreateVariant serialization plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent a Variant from being inserted after its Adoption has ended, including when the service's earlier active-state read is stale.

**Architecture:** `CreateVariant` and `EndAdoption` must enter transactions and issue the same tenant/id/state guarded no-op UPDATE on the Adoption row before their critical work. The database row write is the serialization point: PostgreSQL retains its row lock until commit; SQLite takes its transaction write lock. The service maps a repository state-race sentinel to its existing conflict sentinel.

**Tech Stack:** Go, GORM, SQLite migration-backed repository tests; PostgreSQL locking semantics are accounted for by the shared guarded UPDATE but are not runtime-tested in this round.

**Spec:** `docs/specs/2026-09-20-agent-marketplace-domain-model.md` §9; Issue #63 snapshot and Task 2 brief in `.superpowers/sdd/plan-t63/`.

## Global Constraints

- Tenant predicates must scope every Adoption read/write.
- Ending an Adoption requires every Variant to be retired and must preserve history.
- No non-retired Variant may be committed under an ended Adoption.
- Keep this repair limited to the repository/service Adoption creation and ending seam and regression tests.

## Review Focus

- CreateVariant's service precheck can become stale before insertion; reproduce with a deterministic gate and assert no Variant is inserted after End commits.
- Concurrent create/end transactions must serialize through the same Adoption row; pause the insert after acquiring the guard and verify End cannot pass it.
- A tenant mismatch must not lock or transition another Tenant's Adoption; existing tenant predicates remain in the shared guard.

---

### Task 1: Serialize Variant creation and Adoption ending

**Files:**
- Modify: `internal/application/repository/agent_adoption.go`
- Modify: `internal/application/repository/agent_marketplace_lifecycle.go`
- Modify: `internal/application/service/agent_adoption.go`
- Test: `internal/application/repository/agent_marketplace_lifecycle_test.go`
- Test: `internal/application/service/agent_adoption_test.go`

**Interfaces:**
- Consumes: `AgentAdoptionRepository.CreateVariant` and Task 2 `EndAdoption`.
- Produces: both operations acquire a shared tenant-scoped Adoption state guard inside their transaction; stale CreateVariant requests return `ErrAgentAdoptionStateConflict` and do not insert.

- [x] Write a service regression test that pauses after the active Adoption/Release prechecks and before repository CreateVariant, ends the Adoption, resumes creation, and asserts conflict plus zero Variants.
- [x] Run that test and observe the original bug: CreateVariant succeeds and leaves the post-end Variant.
- [x] Add a shared guarded no-op UPDATE row guard; require CreateVariant to check active and insert in one transaction, and require EndAdoption to acquire the same expected-state guard before counting Variants and transitioning.
- [x] Translate repository state/not-found races to service conflict/not-found sentinels.
- [x] Add a repository interleaving test that holds CreateVariant after acquiring the row guard and proves EndAdoption waits, then rejects ending after the Variant commits.
- [x] Run targeted service/repository tests, repeated SQLite interleaving test, race detector, gofmt and diff check.
- [x] Commit the code checkpoint and produce the BASE-to-HEAD Review Package.
