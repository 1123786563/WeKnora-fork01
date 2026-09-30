# Task 1 Independent Review — Production Admission Wiring

**Range:** `5f121c96df241724428adb2187b9984caf333fb5..11f6da40b1b77fd061207636cf9f6a45f512e852`  
**Scope:** `internal/container/workbench_agent_lifecycle_test.go` only.  
**Sources:** approved marketplace spec §§9–10, ADR 0011, `CONTEXT.md`, Issue #63 AC3 snapshot, Task 1 plan and brief, commit diff, and relevant production code. No source or test files were changed during this review.

## Findings

No actionable findings in this increment.

## Spec compliance: PASS for Task 1

- The test persists a tenant 1 Adoption and a Variant with `state='retired'` and `local_agent_id='retired-agent'` in the full-migration SQLite fixture. `wiringTestDB` creates tenant 1 and session `s-wiring` owned by `u-wiring`.
- It constructs the production `NewWorkbenchAdmissionCoordinator` with the real AgentRunStore, ExecutionTargetStore, and AgentAdoptionRepository, then sends POST through `session.NewWorkbenchStartHandler(coordinator).Start` with Gin tenant and user context. The production container registers both providers (`internal/container/container.go`), and the provider installs `SetAgentUseGate` using `RetiredVariantAgentExists`.
- It asserts HTTP 409 and zero tenant/request-matched `workbench_requests` and `agent_runs`. The handler maps `ErrAgentUseDenied` to 409; the coordinator calls the lifecycle gate before `CreatePending` for a new request. This matches the Task 1 AC3 evidence requirement and the spec's prohibition on new Task creation for retired Variants.

## Code quality: PASS

- The new test stays in `package container`, avoiding the router/container import cycle. It does not install a copied gate or mock the coordinator. The diff touches only the owned test file; the reviewed checkout was clean.
- The gate sensitivity has a concrete assertion: without provider gate registration, `Start` creates a pending `workbench_requests` row before the repository's second retired-Variant check, so the zero-request assertion fails. The implementation report records that exact failure (`found 1`) during a temporary gate removal and records restoration of the provider file. I did not rerun the mutation or verification commands; this verdict uses the diff, implementation report, and read-only source tracing.

**Scope limit:** This verdict is for the Task 1 increment. It does not substitute for the parent lifecycle acceptance review or the controller's OCR pass.
