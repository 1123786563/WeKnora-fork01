# Task 1 Report — T63 Task 6 review findings

Status: DONE_WITH_CONCERNS. F2 is implemented. F1 remains blocked by the Go package import cycle described below.

## Changed files

- `internal/router/routes_agent_marketplace_lifecycle_test.go`
  - Captures the exact seeded variant, releases, submissions, reviews, MIT license, listing, local agent, run, and artifact before lifecycle exits.
  - After all exits, queries those exact identities and checks their key links. Run-to-agent is asserted through its linked workbench request because `agent_runs` has no `agent_id` column in this schema.
  - Retains aggregate counts as supplemental conservation checks.

## Verification evidence

- `go test ./internal/router/ -run 'TestLifecycleExitDeletesNothingAcrossGovernanceRows|TestLifecycleUnlistedAndDeprecatedRemainDistinct|TestLifecycleRetireBlocksNewWorkEndToEnd' -count=1` — PASS (`ok .../internal/router 1.578s`).
- `go test ./internal/container/ ./internal/application/service/ ./internal/modules/workbench/service/workbench/ -run 'Admission|AgentUse|Lifecycle' -count=1` — PASS (`container 1.426s`, `application/service 13.683s`, `workbench/service/workbench 7.296s`). The earlier literal path `./internal/workbench/` from the brief does not exist; used the repository's actual workbench service package path.
- `git diff --check` — PASS.
- Initial baseline focused router command passed before edits (`ok .../internal/router 2.726s`).
- Production-provider attempt: importing `internal/container` in this `package router` test causes `router -> container -> router import cycle not allowed in test`. The change was not retained.

## F1 remaining risk / blocker

The requested same-file construction through `container.NewWorkbenchAdmissionCoordinator` cannot compile from this package because `internal/container` imports `internal/router`. The brief forbids edits to any other file and also disallows falling back to a manually installed gate, so this task cannot satisfy F1 under its current file ownership. The existing `newRealAgentUseGate` / `SetAgentUseGate` path remains in the test. A follow-up must permit either moving this test to an external `router_test` package (with its dependent helpers addressed) or adding a container-owned test seam/file.

## Commit and diff range

- Commit: `707b0f803` (`test: assert lifecycle governance history rows`)
- Base: `0d99b972eaedfaf6aa23a776e798d81d85206e57`
- Changed source files: only `internal/router/routes_agent_marketplace_lifecycle_test.go` (76 insertions).
- Report file is this file.
