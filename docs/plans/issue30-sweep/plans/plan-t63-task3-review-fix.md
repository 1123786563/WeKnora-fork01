# T63 Task3 Review Fix — Adoption terminality and release-use serialization

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. This repair wave addresses the independent review of Task3 commit `c28e783bb3010d748e79779391b65ade097ed4d9`.

**Goal:** Make ended Adoptions terminal and serialize Release deprecation with successor validation and writes that create new use of a Release.

**Findings:** R1 HIGH: after EndAdoption, AdoptListing can return success for the same Release or mutate the ended row to another Release. R2 MEDIUM: simultaneous A→B and B→A deprecations can form a successor cycle. R3 MEDIUM: service pre-read of DeprecatedAt races with Adopt/CreateVariant/Accept writes. R4 LOW coverage: no active-Adoption AcceptUpgradeProposal test rejects a deprecated target.

**Architecture ruling:** Two serial repair tasks because both touch adoption repository state. Task1 serializes `(tenant_id, listing_id)` before reading/reconciling and rejects `ended` on existing-row and unique-conflict reread paths; cover public IntroduceRelease and rollback. Task2 locks source/successor Releases in stable ID order, re-reads and validates inside the transaction, and gates final Adopt/CreateVariant/Accept writes in their repository transaction. Deterministic file-backed SQLite/two-connection/callback tests; no sleeps. Preserve reviewed T63 Task2 EndAdoption/CreateVariant state guard. Reports must distinguish callback interleaving from direct driver lock-wait observation; PostgreSQL only if DSN exists.

## Global Constraints

- Existing isolated `codex/issue30-t63` worktree, base `c28e783bb3010d748e79779391b65ade097ed4d9`.
- Task1 and Task2 are serial. Task2 waits for Task1 independent review/validation PASS and integration.
- Single owner for `agent_adoption.go` and `agent_marketplace_lifecycle.go`; no other concurrent implementation may edit these files.
- Do not modify migrations, routers, container, workbench, T55 mobile or T64 security files. Local commits authorized; each task needs report, independent review and affected validator evidence.

## Review Focus

1. Ended Adoption never returns success or changes accepted pointer; public introduction rolls back atomically.
2. Adoption and EndAdoption serialize on tenant/listing; tests cover controlled orderings without sleep.
3. A→B/B→A cannot both commit; successor is revalidated after stable-order locking.
4. Adopt/CreateVariant/Accept enforce deprecated state at repository write transaction, not service pre-read.
5. Add active Adoption + deprecated target Accept test; preserve error precedence and tenant scope.
6. Disclose actual database/lock evidence and PostgreSQL DSN limitations.

## Task 1 — Keep ended Adoption terminal

**Dependency:** Task3 checkpoint `c28e783bb`; R1 HIGH. **Role:** `backend_implementer`; independent reviewer + backend validator. **Worktree:** existing T63 worktree. **Owned files:** repository `agent_adoption.go`, `agent_adoption_test.go`, `public_marketplace.go`, `public_marketplace_test.go`; service `agent_adoption.go`, `agent_adoption_test.go`.

1. **RED:** Test same-Release and different-Release re-Adopt after EndAdoption; require transition conflict and unchanged ended state/pointer. Test public IntroduceRelease against ended Adoption fails and rolls back a newly inserted introduction. Add deterministic file-backed SQLite ordering cases if callback seams allow. Demonstrate old implementation fails.
2. **Implement:** Wrap AdoptListing in a transaction and acquire a no-op write serialization for tenant/listing before scope read. Re-read after lock. Only active rows can reconcile; ended always yields `ErrAgentAdoptionTransition`, including same Release and conflict-loser reread. Apply same guard before reads/writes in both existing/new IntroduceRelease branches so failed introduction is atomic. Map direct AgentAdoptionService.Adopt transition to `ErrAgentAdoptionStateConflict`.
3. **GREEN:** Run focused new repository/public-introduction tests, existing `TestAgentAdoptionRepository*`, `TestPublicMarketplaceRepositoryIntroduceRelease*`, service adoption tests and `git diff --check`. Accurately state callback-vs-driver observation limits.
4. **Commit/review:** Commit only owned files. Task2 cannot start before independent review and validation pass and integration.

**Acceptance:** ended adoption cannot be revived or mutated; public failure leaves no partial row; active re-adoption remains compatible; tenant isolation remains enforced.

## Task 2 — Serialize Release deprecation and all use gates

**Dependencies:** Task1 reviewed checkpoint integrated; R2/R3 MEDIUM and R4 LOW. **Role:** `backend_implementer`; independent reviewer + backend validator. **Owned files:** repository `agent_adoption.go`, `agent_adoption_test.go`, `agent_marketplace_lifecycle.go`, `agent_marketplace_lifecycle_test.go`; service `agent_adoption.go`, `agent_adoption_test.go`, `agent_marketplace_lifecycle.go`, `agent_marketplace_lifecycle_test.go`, `agent_upgrade.go`, `agent_upgrade_test.go`.

1. **RED:** Deterministic file-backed SQLite schedules for A→B/B→A (one winner/no cycle), deprecation vs Adopt/CreateVariant (serialized later write rejects), stale pre-read then repository write rejects, and active Adoption + deprecated target Accept rejects. Repeatable, no sleeps; fail on Task1 checkpoint.
2. **Implement:** `DeprecateRelease` transaction locks source and successor rows in stable lexical ID order, tenant-scoped re-reads both, validates same listing and nondeprecated successor, then CASes source. Adoption mutation paths keep one explained lock order. AdoptListing/CreateVariant perform final tenant-scoped local Release deprecation check inside the same write transaction; introduced Releases use their tenant-scoped introduction row and have no local deprecated flag. Accept uses guarded CreateVariant. Service pre-read only improves early error text; map repository conflicts to existing 409 sentinels.
3. **GREEN:** Run lifecycle/adoption/upgrade tests including real Accept case, concurrency tests repeated `-count=3`, relevant router-compatible tests and `git diff --check`. PostgreSQL only if DSN exists.
4. **Commit/review:** Commit only owned files; Task4/5 and overlapping T64 service/router/admission work remain blocked until independent review and backend validation pass.

**Acceptance:** ended Adoption stays terminal; opposite deprecations cannot create a cycle; writes ordered after deprecation cannot introduce that Release; Accept is directly covered; prior T63 behavior stays green.

**Failure handling:** If deterministic synchronization cannot be observed, keep regression failing and report exact needed seam; never weaken assertions or add sleeps.

## Interface and Scope Preflight

Task1/2 share repository/service state-transition files and are serialized. Task2 consumes Task1 terminality. T64 Task2 `agent_security.go` and Task3 new run-security files are disjoint. T63 router/admission and overlapping T64 service/wiring tasks remain blocked.
