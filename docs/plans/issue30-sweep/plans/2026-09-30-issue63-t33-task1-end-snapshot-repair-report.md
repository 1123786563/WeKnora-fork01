# Issue #63 T33 Task 1 End Snapshot Repair Report

## Result

Implemented EndAdoption as a transaction with three ordered SQL operations: tenant/id/active conditional parent write gate; separate tenant/adoption-scoped count of Variants whose state is not retired; and active-state CAS update of lifecycle metadata. A failed child check rolls the gate back. Not-found and transition errors remain distinguished.

Added an always-on SQLite sequential contract test and a `semantic_integration` PostgreSQL race test using two DB handles. The integration test pauses CreateVariant after its parent gate, starts EndAdoption on the other handle, polls PostgreSQL lock-wait activity to prove End is waiting, then releases Create and checks both the transition error and final rows.

## Verification evidence

- PostgreSQL DSN: `TRPC_TEST_POSTGRES_DSN` absent. The tagged race test was skipped by its explicit environment guard; PostgreSQL READ COMMITTED concurrency remains unverified here.
- `go test ./internal/application/repository -run '^TestAgentAdoptionRepositoryEndRequiresRetiredVariants$' -count=1` — passed before implementation (sequential assertions established existing contract).
- `go test ./internal/application/repository -run '^TestAgentAdoptionRepository(EndRequiresRetiredVariants|Lifecycle|RejectsWritesAfterEnd)$' -count=1` — passed after implementation.
- `go test -tags semantic_integration ./internal/application/repository -run '^TestAgentAdoptionEndWaitsThenSeesCommittedVariant$' -count=1` — command passed; race case skipped because DSN is absent.
- `go test ./internal/database -run 'TestAgentMarketplaceLifecycleMigration|TestMigrationVersion' -count=1` — passed; includes SQLite lifecycle migration up/down/up.
- `git diff --check` — passed.

## Files and hashes

- `internal/application/repository/agent_adoption.go` — SHA-256 `186902bada1e6b24bfb911699d194c2ed8115ba364190adaded3d5d65db9d256`
- `internal/application/repository/agent_adoption_test.go` — SHA-256 `149b45124f0002a7ce7fc160aed516d9f8bdb1706e85bdc7993d79c687356435`
- `internal/application/repository/agent_adoption_end_concurrency_pg_test.go` — SHA-256 `3fe4612ccbe75067abbade409b9282e2506763b1744d9396642bf258a29cf4cb`

## Limitations / status

The PostgreSQL race is encoded but has no execution evidence until run with a configured `TRPC_TEST_POSTGRES_DSN`. SQLite is only evidence for sequential lifecycle behavior. T33-1 remains pending independent validation/review; no migration was needed for this query-only repair.
