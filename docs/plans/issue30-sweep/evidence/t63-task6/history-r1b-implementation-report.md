# Task 1 Report — T63 history provenance R1

Status: DONE

## Changed file

- `internal/router/routes_agent_marketplace_lifecycle_test.go`
  - Loads the exact Adoption before lifecycle exits, then verifies its ListingID and AcceptedReleaseID are unchanged and its state is `ended`.
  - Verifies the Variant retains its exact pinned ReleaseID and that the pin refers to one of the retained Release rows.
  - Verifies each Release still points to its exact Submission and immutable AgentVersion source row.
  - Decodes each Release manifest and verifies its `license_id` points to the exact seeded license registry row. This remains an original Release fixture; no Fork lineage values were fabricated.

## RED → GREEN evidence

- RED command: `go test ./internal/router/ -run 'TestLifecycleExitDeletesNothingAcrossGovernanceRows' -count=1` — failed as expected at the temporary deliberately incorrect Variant.ReleaseID sentinel (`expected deliberate-red-sentinel`, actual seeded Release UUID). The sentinel was immediately replaced with the captured exact ReleaseID assertion.
- GREEN command: `go test ./internal/router/ -run 'TestLifecycleExitDeletesNothingAcrossGovernanceRows|TestLifecycleUnlistedVersusDeprecatedBehaviorDiffers|TestLifecycleRetireBlocksNewWorkEndToEnd' -count=1` — PASS (`ok .../internal/router 3.519s`).
- `git diff --check` — PASS.

## Commit and range

- Base: `707b0f803dfd3c7cf31271de1b1acdda5dab1fe7`
- Commit: `e033c95eaebbcfce2e257ae75a4e33510c75fb37` (`test: verify lifecycle history provenance`).
- Only source file changed: `internal/router/routes_agent_marketplace_lifecycle_test.go`.
- No remaining concerns for this R1 provenance scope.
