# T63 Task 3 review repair — round 2 report

## Scope and implementation

Implemented both round-2 lifecycle race repairs against base `bef94987e78084bd1008a512762a12f2defe8a3e`:

- `AgentAdoptionRepository.AdoptListing` now wraps eligibility guards and `adoptListingTx` in one transaction. `IntroduceRelease` retains its existing transaction-bound helper path and does not nest transactions.
- Accepted proposal transitions now run in a transaction. The repository loads the tenant-scoped proposal and matching adoption, applies the existing listing/release lifecycle row guards, and performs the proposal state CAS before commit. Missing or stale eligibility returns the existing proposal transition sentinel; the service maps it to the existing upgrade state conflict while preserving both errors in the chain.
- Deterministic channel-gated tests cover Adopt vs Unlist and upgrade acceptance vs Unlist after draft Variant creation. The latter confirms the allowed orphan draft remains draft while the proposal stays open and has no accepted Variant reference.

Files changed:

- `internal/application/repository/agent_adoption.go`
- `internal/application/repository/agent_marketplace_lifecycle_test.go`
- `internal/application/repository/agent_upgrade.go`
- `internal/application/service/agent_upgrade.go`
- `internal/application/service/agent_upgrade_test.go`

## Verification evidence

- `gofmt -w internal/application/repository/agent_adoption.go internal/application/repository/agent_upgrade.go internal/application/repository/agent_marketplace_lifecycle_test.go internal/application/service/agent_upgrade.go internal/application/service/agent_upgrade_test.go` — passed.
- `go test ./internal/application/repository/ -run 'TestAdoptListing(SerializesWithConcurrentUnlist|RechecksListingAndReleaseAtWriteBoundary)|TestFindOrCreateProposalRechecksLifecycleEligibility' -count=1 -timeout=90s` — passed (`ok`, 11.895s).
- `go test ./internal/application/service/ -run 'TestAcceptUpgradeProposal(RechecksLifecycleAfterVariantCreation|RejectsConcurrentUnlist)$' -count=1 -timeout=120s` — passed (`ok`, 4.413s).
- `go test -race ./internal/application/repository/ -run 'TestAdoptListingSerializesWithConcurrentUnlist|TestConcurrentReciprocalDeprecationsCannotPersistSuccessorCycle' -count=1 -timeout=120s` — passed (`ok`, 4.840s).
- `go test -race ./internal/application/service/ -run 'TestAcceptUpgradeProposalRechecksLifecycleAfterVariantCreation$' -count=1 -timeout=120s` — passed (`ok`, 4.867s).
- `go test ./internal/application/repository/ -run 'Test(EndAdoptionRequiresAllVariantsRetiredAndIsTransactional|CreateVariantAndEndAdoptionSerializeOnAdoptionRow|TransitionListingStateIsCAS|DeprecateReleaseIsCASAndPointsAtSuccessor|AdoptListingRechecksListingAndReleaseAtWriteBoundary|CreateVariantRechecksListingAndReleaseAtWriteBoundary|FindOrCreateProposalRechecksLifecycleEligibility|ConcurrentReciprocalDeprecationsCannotPersistSuccessorCycle|AdoptListingSerializesWithConcurrentUnlist)$' -count=1 -timeout=120s` — passed (`ok`, 11.795s).
- `go test ./internal/application/service/ -run 'Test(AgentAdoption|AgentUpgrade|AgentMarketplaceLifecycle|AcceptUpgradeProposal)' -count=1 -timeout=150s` — passed (`ok`, 16.764s).
- `git diff --check` — passed.

An initial test attempt passed through the gated lifecycle operations but exposed a test cleanup double-close panic. The gate now uses `sync.Once`; subsequent focused and race runs passed.

## Review package and commit

- Exact implementation patch from repair base to source files: `.superpowers/sdd/plan-t63/task-3-fix-round2.patch`
- SHA-256: `1c1ad2b4fc8e4de0f2ff1e400d0f4d008da51ff6f90883bf82b0202947e637da`
- The patch excludes plan/report metadata and includes the five source/test files listed above.
- Implementation commit: `fix(marketplace): make adoption eligibility atomic` (recorded on the assigned branch).

## Limits

Verification used the production SQLite migration stream. PostgreSQL transaction/row-lock behavior was not exercised against a live PostgreSQL instance. The accepted Variant is still created before proposal CAS by design; if lifecycle eligibility loses afterward, the proposal remains open and the draft Variant remains as the explicitly allowed benign orphan.
