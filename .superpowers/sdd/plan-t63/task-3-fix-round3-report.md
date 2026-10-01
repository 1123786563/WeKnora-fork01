# T63 Task 3 review repair — round 3 report

## Scope

Implemented round-3 F1–F3 against repair base `1cbfea5fbb57badf936f71354a4906abdfb11bbf`.

- Accepted proposal CAS now validates source adoption state in its own transaction after draft Variant creation: it locks lifecycle rows in listing → target release → adoption order, then requires matching tenant-scoped adoption to remain active, belong to the proposal listing, and still accept `FromReleaseID`.
- Known lifecycle conflict/missing-row sentinels map to `ErrAgentUpgradeProposalTransition`. Other SQL/locking failures are returned unchanged.
- The existing CAS fixture now creates its tenant listing, both Releases, and matching active Adoption.
- Added deterministic service interleavings for Adoption ending and accepted pointer advancement after draft creation, plus repository injection coverage for preserving a non-conflict SQLite storage error.
- The draft Variant remains a separate write; a failed final guard leaves the proposal open with no accepted Variant reference. The end interleaving retires the draft before ending Adoption so the lifecycle precondition is satisfied.

Changed code/test files:

- `internal/application/repository/agent_upgrade.go`
- `internal/application/repository/agent_upgrade_test.go`
- `internal/application/service/agent_upgrade_test.go`

## RED → GREEN evidence

- Before the repository implementation change, `TestAgentUpgradeRepositoryTransitionProposalPreservesStorageError` failed because acceptance incorrectly succeeded despite an injected Adoption guard SQL error.
- Before the implementation change, both new service interleavings failed because acceptance incorrectly succeeded after Adoption end or accepted Release advancement.
- After the change, all three passed; the repository error test confirms the original injected storage error text remains visible and is not mapped to the proposal conflict sentinel.

## Verification

- `gofmt -w internal/application/repository/agent_upgrade.go internal/application/repository/agent_upgrade_test.go internal/application/service/agent_upgrade_test.go` — passed.
- `go test ./internal/application/repository/ -run 'TestAgentUpgradeRepositoryTransitionProposal(CAS|PreservesStorageError)$' -count=1 -timeout=90s` — passed.
- `go test ./internal/application/service/ -run 'TestAcceptUpgradeProposal(RechecksAdoptionEndAfterVariantCreation|RechecksAcceptedReleaseAfterVariantCreation|RechecksLifecycleAfterVariantCreation|RejectsConcurrentUnlist|MapsConcurrentAdoptionEndToConflict)$' -count=1 -timeout=150s` — passed.
- `go test ./internal/application/repository/ -run 'Test(EndAdoptionRequiresAllVariantsRetiredAndIsTransactional|CreateVariantAndEndAdoptionSerializeOnAdoptionRow|TransitionProposalCAS|TransitionProposalPreservesStorageError|AdoptListingRechecksListingAndReleaseAtWriteBoundary|FindOrCreateProposalRechecksLifecycleEligibility|ConcurrentReciprocalDeprecationsCannotPersistSuccessorCycle)$' -count=1 -timeout=150s` — passed.
- `go test ./internal/application/service/ -run 'Test(AgentAdoption|AgentUpgrade|AgentMarketplaceLifecycle|AcceptUpgradeProposal)' -count=1 -timeout=180s` — passed.
- `go test -race ./internal/application/repository/ -run 'TestAgentUpgradeRepositoryTransitionProposal(CAS|PreservesStorageError)$' -count=1 -timeout=150s` — passed.
- `go test -race ./internal/application/service/ -run 'TestAcceptUpgradeProposal(RechecksAdoptionEndAfterVariantCreation|RechecksAcceptedReleaseAfterVariantCreation|RechecksLifecycleAfterVariantCreation)$' -count=1 -timeout=180s` — passed.
- `git diff --check` — passed.

## Patch and commit

- Exact source/test patch from repair base: `.superpowers/sdd/plan-t63/task-3-fix-round3.patch`
- SHA-256: `6ad05a121d840ccfe8428204cfab3ff065fadc719e598184fbbd3649bab35d87`
- This patch contains only the three code/test files listed above; the report and patch metadata are excluded.
- Implementation commit: recorded after report preparation.

## Limits

Verification used SQLite, including deterministic trigger error injection. No live PostgreSQL lock/transaction test was run.
