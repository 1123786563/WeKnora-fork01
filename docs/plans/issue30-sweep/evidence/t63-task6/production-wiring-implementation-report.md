# Task 1 Report — Production-composed retired-Agent admission test

## Result

Implemented and committed the production-provider HTTP integration test. The test uses `wiringTestDB(t)`, persists an adoption/retired variant, constructs `NewWorkbenchAdmissionCoordinator` with the real repositories, invokes `session.NewWorkbenchStartHandler(...).Start`, and asserts HTTP 409 plus zero tenant/request-matched `workbench_requests` and `agent_runs` rows.

## Changed files

- `internal/container/workbench_agent_lifecycle_test.go` (new; sole committed file)

Commit: `11f6da40b1b77fd061207636cf9f6a45f512e852` (`test workbench retired agent production admission wiring`). Worktree is clean after commit. Production `internal/container/workbench.go` is unchanged.

## TDD and sensitivity evidence

- With production wiring present, the new test passed.
- Temporarily removed only the `SetAgentUseGate` registration in `internal/container/workbench.go`. The test then failed at the zero-request assertion (`denied retired-agent start must not create a durable request`, found 1), proving it detects missing gate wiring.
- Restored `workbench.go` from an exact backup. SHA-256 before and after restore matched: `2e73684b776931494c31d28a448c43b715d84f2c2d37ef5d8d539b9c9e491782`.
- Restored production wiring, reran the test, and it passed.

## Verification commands

- `go test ./internal/container/ -run TestWorkbenchAdmissionRejectsRetiredAgentFromProductionProvider -count=1` — PASS.
- `go test ./internal/container/ ./internal/modules/workbench/service/workbench/ -run 'WorkbenchAdmission|AgentUse' -count=1` — PASS (`container` and `workbench` packages).
- `go build ./...` — PASS. Linker emitted existing duplicate `-lc++` library warnings for `cmd/server` and `cmd/desktop`; no build errors.
- `git diff --check` — PASS.

The expected-failure mutation command was the same targeted test command after removing the provider gate; it failed specifically because one durable request was created. The provider source was restored byte-for-byte before final verification and commit.

## Assumptions / remaining risks

- `platform` is the intended trusted built-in target for this admission test, as specified by the Task Brief; the request therefore needs no execution-target fixture.
- No outstanding implementation risk identified. Independent task review and any parent-level integration checks remain for the orchestrator.
