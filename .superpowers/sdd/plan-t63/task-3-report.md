# Task 3 Report — Marketplace lifecycle service and deprecation gates

## Scope and checkpoint

- Worktree: `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t63-task3`
- Branch: `codex/issue30-b6-t63-task3`
- BASE: `0fa8c3d162332dfe532e2adf2556f9622ffccf8a`
- Implementation HEAD: `1897d6491` (`feat(marketplace): add lifecycle service and deprecated release gates`)
- Full review package: `.superpowers/sdd/plan-t63/review-package-task-3-0fa8c3d..a51b354.patch`
- Review package SHA-256: `7af8d4d1457bce77b98808bff8931f47c457300b6e704c7c14c1cc41977d925a`
- Touched package paths: `internal/types/interfaces`; `internal/application/service`
- Runtime role/model: runtime did not expose role/model metadata.

## Delivered

- Added `AgentMarketplaceLifecycleService` interface and service operations for variant retirement, adoption ending, listing unlisting, and release deprecation with successor validation.
- Retire and end use Task 2 tenant-scoped repository CAS primitives. Adoption end precondition errors map to the existing adoption state conflict sentinel.
- Added deprecated release gates to Adopt, CreateVariant, and AcceptUpgradeProposal; reconciliation skips deprecated target releases.
- Added real-migration SQLite service tests. Test fixtures create the schema-required version and submission rows and keep foreign keys enabled.

## Verification evidence

- RED: `go test ./internal/application/service/ -run 'TestRetireVariantIsCAS|TestEndAdoptionGate|TestUnlistAndDeprecate|TestDeprecateSuccessor|TestUpgradeReconcileSkipsDeprecated' -count=1` initially failed at compile time with the expected undefined lifecycle service and sentinels.
- Fixture iteration: first GREEN compilation exposed foreign-key prerequisites; a later run exposed required distinct version numbers and valid upgrade bundle JSON. Fixtures were corrected without disabling constraints.
- PASS: `go test ./internal/application/service/ -run 'TestRetireVariantIsCAS|TestEndAdoptionGate|TestUnlistAndDeprecate|TestDeprecateSuccessor|TestUpgradeReconcileSkipsDeprecated' -count=1` → `ok github.com/Tencent/WeKnora/internal/application/service 5.947s`.
- PASS: `go test ./internal/application/service/ -run 'AgentAdoption|AgentUpgrade' -count=1` → `ok github.com/Tencent/WeKnora/internal/application/service 7.824s`.
- PASS: `go test -race ./internal/application/service/ -run 'TestRetireVariantIsCAS|TestEndAdoptionGate|TestUnlistAndDeprecate|TestDeprecateSuccessor|TestUpgradeReconcileSkipsDeprecated' -count=1` → `ok github.com/Tencent/WeKnora/internal/application/service 11.494s`.
- PASS: `gofmt -d` over the five owned Go files produced no output.
- PASS: `git diff --check` produced no output.

The service-level retired mapping assertion checks `ErrAgentAdoptionStateConflict`: the sequential service call is rejected by its state precheck. The repository remap CAS sentinel is reachable if a concurrent state change occurs after that precheck; the existing service race coverage retains that repository behavior.

## Content hashes (SHA-256 at implementation HEAD)

- `internal/types/interfaces/agent_marketplace_lifecycle.go`: `24262003989a0266c0775d90db688d3fa10d97010ab2de271086cc93ffe117c2`
- `internal/application/service/agent_marketplace_lifecycle.go`: `9a3d05256bc25de77eb1d2b1e49614b632e08a06dcb72288fc01252c8ed234de`
- `internal/application/service/agent_marketplace_lifecycle_test.go`: `1a14b62f2f46a69497264a9136c72ee69f64cb0c3b8f5799cba51a2c2940373f`
- `internal/application/service/agent_adoption.go`: `c1601c5a6315d7fac842f60355a7f47fe230362a0a4108bff46e35da2772daa4`
- `internal/application/service/agent_upgrade.go`: `e6ff775b49f047d76acadd33b0f853baaa898fab188b986e092aac15689dfd1f`

## Limitations and remaining risks

- The tests exercise application service/repository behavior over the real migration stream; they do not replace the assigned Task 6 HTTP end-to-end evidence.
- `paseo.json` was present as unrelated untracked worktree content before implementation and remains untouched.
