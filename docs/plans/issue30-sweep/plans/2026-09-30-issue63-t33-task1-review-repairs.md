# Issue #63 T33 Task 1 Lifecycle Concurrency Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the repository-level lifecycle races found in T33-1 review so ended Adoptions cannot gain active Variants or accepted Releases, and unlisted Listings cannot create new Adoptions.

**Architecture:** Put durable write gates inside the repository transactions. The gate is a conditional no-op `UPDATE` of the same parent row whose lifecycle transition is controlled: active Adoption for variant creation/end and listing state for adoption/introduction/unlist. PostgreSQL serializes on the row; SQLite serializes its writer. Then perform dependent reads and writes in that transaction, check affected-row counts, and keep a consistent Listing → Adoption lock order.

**Tech Stack:** Go, GORM, PostgreSQL versioned migrations, SQLite migrations, existing repository transaction layer.

**Spec:** Issue #63 snapshot and approved Marketplace domain spec §§8–10; ADR-0011; `CONTEXT.md` Listing and Adoption lifecycle; original plan `docs/plans/issue30-sweep/plans/2026-09-30-issue63-t33-lifecycle.md`; review `docs/plans/issue30-sweep/plans/2026-09-30-issue63-t33-task1-independent-review.md`.

## Global Constraints

- Repository writes, not service prechecks, enforce lifecycle invariants.
- Keep tenant Listing / tenant Adoption and public Listing / tenant Adoption identities separate.
- Preserve append-only introduction and Release content; only existing lifecycle/accepted pointer fields may change.
- Never resurrect an ended Adoption or allow new Adoption after the relevant Listing becomes unlisted.
- Use bounded whole-transaction retry only for transient SQLite writer conflicts; never retry an isolated failed statement.
- Do not change the public API or migrations unless the tests prove schema support is required.

## Review Focus

- **CreateVariant vs EndAdoption:** if creation wins, EndAdoption must fail until retirement; if end wins, creation must fail; no committed ended Adoption may have a non-retired Variant.
- **Tenant Unlist vs AdoptListing:** a new Adoption must not commit after the tenant Listing is unlisted.
- **Public Unlist vs IntroduceRelease:** no introduction or Adoption may commit after the platform public Listing is unlisted.
- **Ended Adoption reconciliation:** same Release is not successful re-adoption; a different Release cannot advance `accepted_release_id` after end.
- **Transaction retry:** SQLite busy/snapshot errors retry the entire transaction with a bounded policy or surface a lifecycle conflict without partial writes.

---

### Task 1: Serialize lifecycle gates across variant, adoption, and listing transitions

**Dependencies:** T33-1 implementation commit `24c19ee1c476e9eeaa98df1796ed85dd91b8baa7` and independent findings F1–F3.

**Owner role:** `backend_implementer`. **Validator role:** `backend_validator`. **Independent reviewer:** `reviewer`.

**Files:**
- Modify: `internal/application/repository/agent_adoption.go`
- Modify: `internal/application/repository/public_marketplace.go`
- Test: `internal/application/repository/agent_adoption_test.go`
- Test: `internal/application/repository/public_marketplace_test.go`
- Extend only if needed: `internal/database` PostgreSQL repository concurrency test harness following `semantic_migration_pg_test.go`

**Interfaces:**
- Consume existing repository APIs `CreateVariant`, `EndAdoption`, `AdoptListing`, `UnlistTenantListing`, `UnlistPublicListing`, and `IntroduceRelease`.
- Produce serializable database write gates: a conditional no-op update of the parent row in the same transaction precedes dependent reads/writes; affected-row count zero returns the existing lifecycle conflict/not-found error.
- Preserve existing API interfaces and lifecycle migration schema.

- [ ] **Step 1: Add failing lifecycle race and regression tests.** Add direct-create-under-ended-Adoption rejection; tenant/public same- and different-release re-adopt after end rejection; tenant and public unlist/adopt rejection; and controlled two-transaction tests for create/end and unlist/adopt winner orders. Assert final rows and zero post-unlist Adoption/Introduction. Use file-backed SQLite with independent connections for repository integration; add PostgreSQL tests on the existing `TRPC_TEST_POSTGRES_DSN` harness to prove row-lock behavior under concurrent transactions.
- [ ] **Step 2: Run focused tests to establish the race gaps.** Run `go test ./internal/application/repository -run 'Test(AgentAdoption|AgentLifecycle|PublicMarketplace).*Lifecycle|Test.*Unlist.*Adopt|Test.*End.*Variant' -count=1 -v` and the focused PostgreSQL test with `TRPC_TEST_POSTGRES_DSN` when configured. Expected before repair: ended Adoption may accept a new Variant/Release or an adoption transaction may pass the stale listed read; the PostgreSQL contention case exposes the missing row gate.
- [ ] **Step 3: Add active Adoption write gates.** In a transaction, conditionally update the tenant-scoped parent Adoption with `state='active'` before CreateVariant; require one affected row. Make EndAdoption acquire the same gate, check `NOT EXISTS` for non-retired Variants, then set `ended`. For adopt/reconcile, gate existing Adoption active even for same-release return; make pointer advancement a `state='active'` CAS and reject zero affected rows.
- [ ] **Step 4: Add matching tenant/public Listing write gates.** In AdoptListing, transactionally gate the tenant `agent_marketplace_listings` row as `listed` before adoption writes. In IntroduceRelease, first gate the platform `public_marketplace_listings` row as `listed`, then the adopter Adoption row, then write the introduction/adoption. Make tenant/public Unlist CAS update those exact same Listing rows. Keep lock order Listing → Adoption and validate listing/release identity before dependent writes.
- [ ] **Step 5: Bound transient SQLite transaction retries.** If the file-backed race tests show `SQLITE_BUSY`/`SQLITE_BUSY_SNAPSHOT`, retry the complete transaction with bounded backoff and deterministic cancellation; never resume from a partially failed statement. If current GORM transaction behavior already serializes/gates correctly without retry, record the evidence and do not add a retry wrapper.
- [ ] **Step 6: Run focused repository and migration verification.** Run the lifecycle/adoption/public repository tests, SQLite migration up/down/up test, migration uniqueness test, and `git diff --check`. Run PostgreSQL race tests when `TRPC_TEST_POSTGRES_DSN` exists; otherwise record this as an environment limitation without substituting SQLite as proof of PostgreSQL row locking.
- [ ] **Step 7: Commit and report.** Commit only the owned repository/test files and report all winner-order evidence, SQL dialect results, diff hash, and any unavailable PostgreSQL run at `docs/plans/issue30-sweep/plans/2026-09-30-issue63-t33-task1-review-repair-report.md`.

**Failure handling:** If SQLite and PostgreSQL require materially different lock/retry behavior or an invariant cannot be enforced without schema change, stop before introducing dialect-specific SQL and record the exact constraint; update the plan after an architecture review. Do not replace durable database gates with service checks or in-memory locks.

## Plan self-review

- Finding coverage: F1, F2, and F3 each map to explicit repository behavior and final-state assertions.
- Interface consistency: existing repository signatures remain intact; downstream Services, admission, and HTTP tasks can consume them unchanged.
- Dependency order: one serial task owns both repository files because transaction lock order spans tenant and public adoption paths.
- Verification: SQLite validates local transactional behavior; PostgreSQL contention tests are required for row-lock evidence when the DSN is available.
