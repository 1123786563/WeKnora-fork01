# Task 2 Report — repository lifecycle primitives

## Scope

Implemented only T33 Task 2 from `task-2-brief.md`: transactional Adoption termination, tenant scoped retired Variant lookup, compare-and-set Listing transitions, and compare-and-set Release deprecation. Added the repository interface declarations, sentinels, and migration-backed repository behavior tests.

## Changed files

- `internal/application/repository/agent_marketplace_lifecycle.go`
- `internal/application/repository/agent_adoption.go`
- `internal/application/repository/agent_marketplace.go`
- `internal/application/repository/agent_marketplace_lifecycle_test.go`

The existing untracked `paseo.json` was left untouched.

## Behavior and evidence

- `EndAdoption` reads and validates the tenant Adoption in a transaction, counts non-retired Variants, reports the count and first residual Variant ID in the precondition error, and only then applies an expected-state guarded update. End actor/time and updated time are persisted. Missing, stale-state, and residual-Variant cases have distinct sentinels.
- Listing state updates are tenant scoped and CAS guarded. A lost CAS re-reads within tenant scope to distinguish not found from state conflict.
- Release deprecation is tenant scoped and guarded by `deprecated_at IS NULL`; a lost CAS distinguishes not found from already deprecated. The complete row is returned after a successful write.
- Retired local-Agent lookup includes tenant and `state='retired'` predicates.
- The new tests apply the actual SQLite migration stream and cover end preconditions/rollback, successful lifecycle timestamp updates, repeat-transition conflicts, tenant isolation, release successor metadata, and retired-agent lookup. A lifecycle update cannot move or rename its row through `updates`: `id`, `tenant_id`, and `created_at` are filtered; state and updated timestamp are controlled by the repository.

## Verification

- RED: `go test ./internal/application/repository/ -run 'TestEndAdoption|TestTransitionListingState|TestDeprecateRelease|TestRetiredVariantAgentExists' -count=1` — expected compile failure before interface methods/sentinels existed.
- GREEN: `go test ./internal/application/repository/ -run 'AgentMarketplaceLifecycle|EndAdoption|TransitionListingState|DeprecateRelease|RetiredVariantAgentExists' -count=1` — PASS (`ok`, 17.064s).
- `gofmt` on all four owned Go files — PASS.
- `git diff --check` and `git diff --cached --check` — PASS.
- `go test ./internal/application/repository/ -count=1` — not completed; stopped after about 6m20s. Before interruption it reported failures in `TestDeliveryCollaborationEndToEndAC1PersonalLoopAndAttribution` and `TestDeliveryCollaborationEndToEndAC2CollaboratorCannotInheritPersonalConnection` (HTTP 400 `code_delivery_unsupported_provider`: fixture connection not recognized as a code platform connection). The command ended by interrupt with `signal: interrupt`; this full package run is not a passing result.

## Commit and Review Package

- Code checkpoints: `6346ca0ee8e194e87080f858f5ba2f27660a0752` (`feat(marketplace): add lifecycle repository primitives`) and `81abe4136d654f8e272ab8531dcb9bbe08624f42` (`fix(marketplace): keep lifecycle updates tenant scoped`).
- Review range: `197794496..81abe4136d654f8e272ab8531dcb9bbe08624f42`.
- Exact patch: `.superpowers/sdd/plan-t63/review-package-task-2-81abe4136.patch`.
- SHA-256: `357b2d74e3beaf66ecab87197882638061d2019745a5503262864f13f85e9c0f`.

## Remaining review / risk

Independent task review is still pending. Full repository package verification did not complete and exposed the two delivery collaboration fixture failures above; they are outside this task's changed files, but have not been established against BASE in this task.
