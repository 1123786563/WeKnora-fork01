# T63 Task3 Review Fix — Adoption terminality and Release deprecation serialization

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. This review-fix wave addresses findings from the independent review of T63 Task3 checkpoint `c28e783bb3010d748e79779391b65ade097ed4d9`.

**Goal:** Make ended Adoptions terminal and serialize deprecation with successor validation and writes that create new use of a Release.

**Findings:** R1 HIGH: after `EndAdoption`, `AdoptListing` may return success for the same Release or mutate the ended row to another Release. R2 MEDIUM: simultaneous A→B and B→A deprecation may create a successor cycle. R3 MEDIUM: service pre-read of `DeprecatedAt` can race with subsequent Adopt/CreateVariant/Accept writes. R4 LOW coverage: no actual AcceptUpgradeProposal test rejects a deprecated target on an active Adoption.

**Architecture ruling:** two sequential repair tasks, not parallel workers, because both touch adoption repository state. For Adoption terminality, serialize `(tenant_id, listing_id)` in a transaction before reading/reconciling, and reject `ended` on existing-row and unique-insert-loser paths; cover `IntroduceRelease` helper paths and transaction rollback. For Release deprecation, lock source/successor rows in stable ID order, re-read and validate inside the transaction, then CAS. Final Adopt/CreateVariant/Accept writes must gate deprecated state inside repository transactions; service reads remain early UX only. Tests use deterministic file-backed SQLite/two connections/callback channels, no sleeps. Preserve the existing reviewed T63 Task2 EndAdoption/CreateVariant state guard and disclose callback-vs-driver lock observation limits.

## Global Constraints

- Existing isolated T63 worktree, based on `c28e783bb3010d748e79779391b65ade097ed4d9` on `codex/issue30-t63`.
- Task 1 and Task 2 are serial. Task 2 starts only after Task 1 is independently reviewed/validated and integrated in the same worktree.
- No other agent may concurrently edit `internal/application/repository/agent_adoption.go` or `agent_marketplace_lifecycle.go`.
- T64 security store/runner tasks and T55 mobile work are separate file sets. Do not change their files.
- No migrations/router/container/workbench edits. Local commits authorized; each task requires separate report, independent review and affected validator evidence.

## Review Focus

1. Ended rows never return success or change accepted pointers; public introduction rolls back atomically when adoption is ended.
2. Adoption versus EndAdoption serializes on tenant/listing; test both controlled orderings without sleep.
3. A→B/B→A deprecations cannot both commit; successor is revalidated after stable-order locking.
4. Adopt/CreateVariant/Accept use repository transaction gates; service pre-reads do not carry the invariant.
5. Active Adoption + deprecated `toRelease` is covered by an actual Accept service test; preserve existing conflict sentinels and tenant scope.
6. Report PostgreSQL and direct lock-wait evidence only if actually run/observed.

## Task 1 — Keep ended Adoption terminal

**Dependency:** T63 Task3 checkpoint `c28e783bb`; R1 HIGH.

**Role:** `backend_implementer`; independent reviewer and backend validator. **Worktree:** existing T63 worktree. **Owned files:** `internal/application/repository/agent_adoption.go`, `agent_adoption_test.go`, `public_marketplace.go`, `public_marketplace_test.go`, `internal/application/service/agent_adoption.go`, `agent_adoption_test.go`.

1. **RED:** Add tests for same-Release and different-Release re-Adopt after EndAdoption; assert transition conflict and unchanged ended state/accepted pointer. Add public IntroduceRelease test where an ended Adoption rejects and a newly introduced row is rolled back. Add controlled file-backed SQLite ordering tests if existing callbacks permit: Adopt wins then End closes it, or End wins then Adopt rejects. Show the old implementation fails.
2. **Implement:** Wrap `AdoptListing` in a transaction. Acquire a no-op write serialization on `(tenant_id, listing_id)` before reading/reconciling, then re-read. Existing `active` may reconcile; `ended` always returns `ErrAgentAdoptionTransition`, even for same Release. Keep unique-conflict loser reread guarded. In both `IntroduceRelease` existing and new paths, acquire the same adoption-scope guard before relevant snapshot reads/writes; transaction rollback must remove a just-created introduction if ended adoption blocks it. Map repository transition to `ErrAgentAdoptionStateConflict` in direct `AgentAdoptionService.Adopt`.
3. **GREEN:** Run focused new repository and public-introduction tests, existing `TestAgentAdoptionRepository*` and `TestPublicMarketplaceRepositoryIntroduceRelease*`, service adoption tests, and `git diff --check`. Record any concurrency observation limit.
4. **Commit/review:** Commit only owned files. Do not start Task2 until independent review and validation PASS and this checkpoint is integrated in this worktree.

**Acceptance:** ended adoption cannot be revived or mutated; failed public introduction leaves no partial row; active re-adoption remains compatible; tenant boundaries remain enforced.

## Task 2 — Serialize Release deprecation and all use gates

**Dependencies:** Task1 reviewed checkpoint integrated; R2/R3 MEDIUM and R4 LOW.

**Role:** `backend_implementer`; independent reviewer and backend validator. **Owned files:**
- `internal/application/repository/agent_adoption.go`, `agent_adoption_test.go`
- `internal/application/repository/agent_marketplace_lifecycle.go`, `agent_marketplace_lifecycle_test.go`
- `internal/application/service/agent_adoption.go`, `agent_adoption_test.go`
- `internal/application/service/agent_marketplace_lifecycle.go`, `agent_marketplace_lifecycle_test.go`
- `internal/application/service/agent_upgrade.go`, `agent_upgrade_test.go`

1. **RED:** Deterministic file-backed SQLite schedules for concurrent A→B/B→A deprecations (only one may win, no cycle); deprecation vs Adopt/CreateVariant (a write ordered after deprecation rejects); stale service read followed by repository write rejects; active Adoption + deprecated target `AcceptUpgradeProposal` rejects. No sleeps. Confirm failures against Task1 checkpoint.
2. **Implement repository gates:** `DeprecateRelease` transaction locks source and successor tenant-scoped rows in stable lexical ID order, re-reads both, checks same listing and successor not deprecated, then updates source. Adoption mutation paths keep consistent lock order; explain why no transaction acquires locks in reverse. In `AdoptListing` and `CreateVariant`, final write transaction checks and serializes target local Release's non-deprecated state before changing Adoption/creating Variant. Public tenant-introduced Release uses its tenant-scoped introduction row (no local `deprecated_at` field) and remains an allowed distinct source. `AcceptUpgradeProposal` uses the guarded `CreateVariant` primitive; service pre-read remains only an early message. Map repository conflict to existing service 409 sentinels.
3. **GREEN:** Run concurrency tests and repeat them with `-count=3`, lifecycle/adoption/upgrade tests including actual Accept case, relevant router-compatible tests, `git diff --check`. PostgreSQL only if `TRPC_TEST_POSTGRES_DSN` is available.
4. **Commit/review:** Commit only owned files, then independent review and backend validation. T63 Task4/5 and overlapping T64 service/router/admission tasks stay blocked until both pass.

**Acceptance:** ended Adoption remains terminal; opposite deprecations cannot form a cycle; operations ordered after deprecation cannot introduce that Release; real Accept path is covered; prior verified T63 lifecycle behavior remains green.

**Failure handling:** If deterministic synchronization cannot be observed with current callback seams, retain failing tests and report the precise needed test seam; never weaken assertions or add sleeps.

## Interface and Scope Preflight

Task 1 and Task 2 share repository/service state-transition files and must be serialized. Task2 consumes reviewed terminality behavior from Task1. T64 Task2 (`agent_security.go`) and Task3 (new run-security files) are disjoint. T63 router, admission, and T64 shared service/wiring tasks remain blocked until this repair is independently accepted.
