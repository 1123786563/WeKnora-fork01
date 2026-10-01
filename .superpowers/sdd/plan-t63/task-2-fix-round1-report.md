# Task 2 Repair Round 1 Report — Adoption/CreateVariant serialization

## Scope

Addressed the independent HIGH finding that `AgentAdoptionService.CreateVariant` can pass an active-state precheck, then insert after `EndAdoption` commits. Both repository operations now acquire the same tenant/id/expected-state guarded no-op UPDATE inside their transaction before their critical work. `CreateVariant` checks active Adoption and inserts within that transaction. `EndAdoption` acquires the matching guard before checking residual Variants and changing state. The service translates stale-state and missing-row races to its existing conflict/not-found sentinels.

The no-op guarded UPDATE serializes by the Adoption row: PostgreSQL holds the row lock until commit; SQLite acquires the transaction write lock. SQLite is exercised by migration-backed interleaving tests. No PostgreSQL test database was available in this round.

## Changed files

- `internal/application/repository/agent_adoption.go`
- `internal/application/repository/agent_marketplace_lifecycle.go`
- `internal/application/repository/agent_marketplace_lifecycle_test.go`
- `internal/application/service/agent_adoption.go`
- `internal/application/service/agent_adoption_test.go`

## TDD and verification

- RED: `go test ./internal/application/service/ -run '^TestCreateVariantRechecksAdoptionAfterServicePrecheck$' -count=1` — failed as expected because stale-prechecked CreateVariant returned nil error after EndAdoption committed.
- GREEN: `go test ./internal/application/service/ -run '^TestCreateVariantRechecksAdoptionAfterServicePrecheck$' -count=1` — PASS after the shared row guard and service error translation.
- `go test ./internal/application/repository/ -run '^TestCreateVariantAndEndAdoptionSerializeOnAdoptionRow$' -count=5` — PASS, five SQLite interleaving runs. It pauses creation after the guarded UPDATE, starts EndAdoption, confirms the end guard cannot complete while creation holds the transaction, then confirms EndAdoption sees the committed non-retired Variant and leaves Adoption active.
- `go test ./internal/application/repository/ -run 'TestAgentAdoptionRepositoryLifecycle|TestEndAdoptionRequiresAllVariantsRetiredAndIsTransactional|TestCreateVariantAndEndAdoptionSerializeOnAdoptionRow|TestRetiredVariantAgentExists' -count=1` — PASS.
- `go test ./internal/application/service/ -run 'TestCreateVariantRechecksAdoptionAfterServicePrecheck|TestAgentAdoptionServiceVariantsMappingTestPublish|TestAgentAdoptionService' -count=1` — PASS.
- `go test -race ./internal/application/service/ -run '^TestCreateVariantRechecksAdoptionAfterServicePrecheck$' -count=1` — PASS.
- `gofmt` on changed Go files and `git diff --check` — PASS.
- Full repository package suite was not rerun per the repair assignment; the prior Task 2 report records the earlier interrupted run and two unrelated delivery fixture failures.

## Code checkpoint and Review Package

- Repair BASE: `3b2429119d4b11a6a5b4d4f15238dd65c0be3118`.
- Repair HEAD: `4fed833ab28c3846ede6c8a71260e3fcddbd0603` (`fix(marketplace): serialize variant creation with adoption end`).
- Exact Review Package: `.superpowers/sdd/plan-t63/task-2-fix-round1-review.patch`.
- SHA-256: `09f96650133c775d459e9b4209ddb8dd220a7e96a38c33434c3fb7de22871690`.
- Commit range: `3b2429119d4b11a6a5b4d4f15238dd65c0be3118..4fed833ab28c3846ede6c8a71260e3fcddbd0603`.

## Remaining limits

- The interleaving tests use SQLite's real migration stream. PostgreSQL was not available to run an integration test; the implementation uses a guarded UPDATE, which acquires and retains the matching row lock in PostgreSQL transactions as well.
- Independent repair review is pending.
