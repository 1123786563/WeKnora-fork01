# Task 8A independent review

Reviewed `283022dd30ba9fa5f73a238d1d6c5e6a5d4fa1e9..ffd4bc10f7aaf226b7f3272b91b41212ddde8ac8`, the Task 8A brief and implementation report, the approved Marketplace spec, `CONTEXT.md`, ADR-0011, ADR-0015, and the Task 8 plan. Scope: the two owned files only. This was a read-only source review; I did not rerun tests or OCR. The implementation report records successful runs of the two required focused tests and `git diff --check` at this HEAD.

## Findings

### T64-8A-R1-1 — Medium — stale published mapping admits a deleted local Agent

- **Affected symbol:** `checkLocalAgentReleaseAdmissionTx`, `internal/application/repository/agent_security_guard.go:151-167`.
- **Evidence:** The adopted path checks the Variant state, matching Version row, and Release security predicate, but never checks that `(sourceTenantID, localAgentID)` is a live `CustomAgent`. `CustomAgent` is soft deleted by `customAgentRepository.DeleteAgent` (`internal/application/repository/custom_agent.go:59-62`), while `AgentVersionEntity` deliberately retains historical versions and has no deletion marker (`internal/types/agent_version.go:7-18`). The existing `PublishedAvailableAgents` read explicitly omits a published Variant when its local Agent is soft deleted or missing (`internal/application/repository/agent_adoption.go:367-413`). The new adopted test fixture inserts a Version and Variant but no local `CustomAgent` (`agent_security_guard_test.go:75-93`), and expects admission success (`:25`).
- **Impact:** A stale/deleted local Agent can receive a successful adopted admission verdict and pinned Release. This violates the brief's fail-closed stale association requirement and the approved spec's rule that new Task/Run availability depends on the local Agent state. Downstream callers that rely on this shared predicate could commit a new Run/claim for a deleted Agent.
- **Smallest defensible correction:** In the same guarded transaction, require exactly one live tenant-scoped `CustomAgent` for the adopted path as well as the ordinary path, and return `ErrAgentSecurityReleaseUnresolvable` otherwise. Seed a real live local Agent in successful adopted fixtures; add missing and soft-deleted Agent cases that preserve the Version/Variant/Release rows and assert failure. Keep the same public helper signature and Variant-before-Version lock order.

### T64-8A-R1-2 — Low — required blocked-error contract is not asserted

- **Affected test:** `TestCheckLocalAgentReleaseAdmissionTx`, `internal/application/repository/agent_security_guard_test.go:31-35,63-66`.
- **Evidence:** All rejection cases use only `require.Error`. The plan's Checkpoint 8A explicitly expects a revoked Release to return `ErrAgentSecurityReleaseBlocked`, while missing/mismatched/stale mapping is an unresolvable association. The test would pass if those errors were accidentally swapped or if the exact dependency revocation stopped returning the blocked sentinel.
- **Impact:** A regression in the error category could change downstream HTTP handling of a policy block versus an invalid mapping without breaking this focused test.
- **Smallest defensible correction:** Add the expected sentinel to the table and use `require.ErrorIs` for Release and dependency revocations and for unresolvable association cases. Retain exact Release ID and adopted assertions for successful cases.

## Verdict

- **Spec Compliance: FAIL.** The helper correctly uses the supplied transaction, scopes Variant and Version reads by tenant and local Agent, locks Variant rows in stable ID order, pins the Variant's Release, delegates exact dependency matching to `checkReleaseAdmissionTx`, and rejects duplicate mapping rows. Finding T64-8A-R1-1 leaves a supported stale local-Agent state admissible.
- **Code Quality: FAIL.** The focused test's adopted success fixture masks the missing live-Agent check, and its error-only assertions do not verify the planned policy-block category. No other defect was established in the reviewed delta.

The reported SQLite focused runs support the implemented paths only; this review did not independently validate PostgreSQL row-lock behavior or concurrent admission at this checkpoint.
