# T64 Task 7 Review Fix R1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Implement this single repair task in its isolated worktree, then submit it for an independent Review and backend validation.

**Goal:** Remove T64 Task7's duplicate Dig provider and prove the production security-provider registration path remains resolvable.

**Architecture:** Keep the existing `AgentRunStore` provider in the core repository registration and make the security provider registration consume that singleton. Extract the security provider sequence into one helper used by `BuildContainer`; exercise that exact helper with a Dig test container that already has the run store registered, then resolve the new security handler and both gate consumers.

**Tech Stack:** Go, Uber Dig, GORM, existing internal container/service/repository packages.

**Spec:** `docs/plans/issue30-sweep/plans/plan-t64.md` Task 7; independent finding `T7-R1` in `.superpowers/sdd/plan-t64/task-7-review-report.md` at the Task7 source worktree.

## Global Constraints

- Reuse the existing `repository.NewAgentRunStore` provider; register no second `*repository.AgentRunStore` output.
- Preserve `NewAgentSecurityService`, `NewAgentSecurityHandler`, `wireAgentSecurityGates`, route registration, and the existing single-writer container construction.
- Keep Task7's seven-file implementation scope; the repair may modify only `internal/container/container.go` and `internal/container/agent_security_wiring_test.go`.
- No push, merge, publish, deploy, GitHub Issue edit, or Issue closure.

## Review Focus

- Production Dig registration must not panic on duplicate `*repository.AgentRunStore`; test the same provider helper called by `BuildContainer` with an existing run-store registration.
- The security service and handler must resolve, and the adoption/upgrade gate invoke must resolve in Dig.

---

### Task 1: Deduplicate and verify Task7 Dig registration

**Files:**
- Modify: `internal/container/container.go`
- Modify: `internal/container/agent_security_wiring_test.go`

**Interfaces:**
- Consumes: `repository.NewAgentRunStore`, `repository.NewAgentSecurityStore`, `NewAgentSecurityService`, `NewAgentSecurityHandler`, `wireAgentSecurityGates`.
- Produces: `provideAgentSecurity(*dig.Container)` used by `BuildContainer`; it registers only security-specific providers and gates, and relies on the core `*repository.AgentRunStore` provider.

- [ ] Add a focused Dig test for `provideAgentSecurity`: prepare `dig.New()`, provide a minimal `*gorm.DB`, register `repository.NewAgentRunStore` once, provide zero-value `*service.AgentAdoptionService` and `*service.AgentUpgradeService` constructors, invoke `provideAgentSecurity`, and resolve `*handler.AgentSecurityHandler`. The invocation must succeed without a duplicate-provider panic and must execute the gate wiring invoke.
- [ ] Run `go test ./internal/container/ -run '^TestAgentSecurityProvidersResolveWithExistingRunStore$' -count=1`; expected RED because `provideAgentSecurity` is undefined.
- [ ] Extract the existing Agent Security `Provide`/`Invoke` sequence into `provideAgentSecurity` and call it once from `BuildContainer`; remove the second `Provide(repository.NewAgentRunStore)` because the core registration at line 227 already supplies it.
- [ ] Extend the source wiring assertion to verify `BuildContainer` calls the helper and the helper does not register `repository.NewAgentRunStore`.
- [ ] Run the focused Dig regression, existing `go test ./internal/container/ -run TestAgentSecurityWiringRegistered -count=1`, `go test ./internal/handler/ -run 'TestRevoke' -count=1`, `go build ./...`, and `git diff --check`; expect all pass. The Dig regression must fail if a second run-store provider is temporarily restored.
- [ ] Commit only the two owned files. Record BASE, HEAD, test output, and the Review finding closed.

**Failure handling:** If the focused Dig test cannot resolve a dependency, keep the helper's production provider list intact and add only the minimal constructor stubs to the test container. Do not invoke `BuildContainer`, which initializes unrelated infrastructure; test the exact extracted provider helper instead. Do not weaken duplicate detection or suppress Dig errors.

### Task 2: Assert both governance gates are injected

**Files:**
- Modify: `internal/container/agent_security_wiring_test.go`

**Interfaces:**
- Consumes: `provideAgentSecurity(*dig.Container)`, `service.NewAgentAdoptionService`, `service.NewAgentUpgradeService`, and their private `releaseSecurityGate` fields set by `wireAgentSecurityGates`.
- Produces: `TestAgentSecurityProvidersResolveWithExistingRunStore` proves that Dig resolves nonnil adoption and upgrade services with nonnil release-security gates, in addition to resolving the security handler while reusing the single AgentRunStore provider.

- [ ] Resolve `*service.AgentAdoptionService` and `*service.AgentUpgradeService` in the focused Dig test, then assert both are nonnil and their private `releaseSecurityGate` fields are nonnil using reflection. Keep the existing handler resolution and duplicate-provider behavior. Run the focused test; expect FAIL because its two test providers still return nil services.
- [ ] Change those providers to return nonnil zero-dependency instances from `service.NewAgentAdoptionService(nil, nil, nil)` and `service.NewAgentUpgradeService(nil)`.
- [ ] Run the focused Dig test, `go test ./internal/container/ -run '^TestAgentSecurityWiringRegistered$' -count=1`, and `git diff --check`; expect all pass. Confirm the focused test fails if either `SetReleaseSecurityGate` invocation is temporarily removed from `wireAgentSecurityGates`, then restore production code.
- [ ] Commit only `internal/container/agent_security_wiring_test.go` and report exact BASE/HEAD, outputs, and disposition of `T7-R1-F1`.

**Failure handling:** If reflection cannot inspect the unexported interface field without invoking it, use a small SQL-backed behavioral fixture with a revoked Release and prove `Adopt` and `AcceptUpgrade` each return the security-blocked error before repository mutation. Do not add production-only inspection APIs.
