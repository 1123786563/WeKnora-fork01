# T33 Task 2 Repair Round 2 Report — upgrade accept conflict mapping

## Scope and resolution

Fixed the medium regression where `AcceptUpgradeProposal` can pass its adoption-state precheck, then race with `EndAdoption` before its repository `CreateVariant` call. The guarded repository correctly refuses creation with `repository.ErrAgentAdoptionTransition`; the upgrade service now wraps that sentinel with `ErrAgentUpgradeStateConflict`, preserving both with `%w`. Existing `upgradeClientError` maps the service sentinel to HTTP 409, so no handler source change was needed.

A gated, migration-backed service test pauses Accept at `CreateVariant` after proposal/adoption/release prechecks, ends the Adoption through the repository, then resumes Accept. It asserts both error sentinels, zero Variants, and that the proposal remains open.

## Changed files

- `internal/application/service/agent_upgrade.go`
- `internal/application/service/agent_upgrade_test.go`

Plan: `.superpowers/sdd/plan-t63/task-2-fix-round2-plan.md`.

## Verification

- RED: `go test ./internal/application/service/ -run '^TestAcceptUpgradeProposalMapsConcurrentAdoptionEndToConflict$' -count=1` — failed as expected: returned error chain contained only `agent adoption state transition failed`, not `ErrAgentUpgradeStateConflict`.
- GREEN/focused behavior: `go test ./internal/application/service/ -run 'TestAcceptUpgradeProposalMapsConcurrentAdoptionEndToConflict|TestAgentUpgradeServiceResolvesAndRefusesOutOfStateOperations|TestAgentUpgradeServiceAcceptRaceLoserLeavesBenignOrphanDraft' -count=1` — PASS.
- Relevant upgrade suite: `go test ./internal/application/service/ -run 'TestAgentUpgradeService|TestAccept' -count=1` — PASS (`ok`, 13.392s).
- Race check: `go test -race ./internal/application/service/ -run '^TestAcceptUpgradeProposalMapsConcurrentAdoptionEndToConflict$' -count=1` — PASS (`ok`, 6.822s).
- `gofmt` and `git diff --check` — PASS.
- Handler mapping was inspected: `upgradeClientError` maps `ErrAgentUpgradeStateConflict` to `apperrors.NewConflictError`. No handler test exists for this helper and no handler code changed; the deterministic regression exercises the service's error chain that activates the existing 409 mapping.
- No PostgreSQL test database was available. This round only changes service sentinel translation; repository database locking was implemented and tested in repair round 1.

## Review verdict

Independent review verdict: **PASS**, communicated by the parent coordinator after review of the exact round-2 patch. The separate reviewer report file/hash was not exposed in this worktree. Reviewed artifact identity is the patch SHA-256 below.

## Checkpoint and exact Review Package

- Repair round 2 BASE: `7dd4074323c658d51c3012a87533d260fc1066cb`.
- Code HEAD: `1d957470b2032cc876924d469f90ef8ca0fc1668` (`fix(marketplace): map upgrade adoption race to conflict`).
- The plan, report, and progress update are included in a docs-only checkpoint following the code checkpoint; the documentation commit SHA is reported to the coordinator.
- Exact patch: `.superpowers/sdd/plan-t63/task-2-fix-round2-review.patch`.
- SHA-256: `2a5400cd43df81250bcc9478bfcd07c64bf2fade4cb1b42345f0eb72c41c63dc`.
- Range: `7dd4074323c658d51c3012a87533d260fc1066cb..1d957470b2032cc876924d469f90ef8ca0fc1668`.

## Runtime identity

This session exposed no agent metadata API for the exact runtime model or configured `agent_type`. Work was executed as the assigned backend implementation subagent; exact model ID is unavailable here.
