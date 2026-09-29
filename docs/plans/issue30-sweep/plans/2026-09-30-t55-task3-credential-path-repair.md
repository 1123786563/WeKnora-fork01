# T55 Task 3 Managed Credential Evidence Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Strengthen Task 3’s integration test so the fake managed Delivery credential is persisted and resolved through the production credential resolver while the same tenant’s general Shell is created and executed.

**Architecture:** Keep the existing production Delivery HTTP dispatcher and tenant Shell resolver. Replace only the Delivery fixture’s hard-coded credential source with `MCPOAuthBindingStore` seeded with a fake OAuth token and the production `CredentialResolver`; keep HTTP and remote sandbox fakes at their existing outbound seams. Confine Authorization capture to an injected HTTP client instead of mutating `http.DefaultClient`.

**Tech Stack:** Go, GORM/SQLite, Gin HTTP test harness, existing app connector OAuth credential store and resolver.

**Spec:** `docs/plans/issue30-sweep/plans/2026-09-30-t55-integrated-review-repairs.md` Task 3; finding `T55-T3-1` in `.superpowers/sdd/plan-t55-integrated-review-repairs/task-3-independent-review.md`.

## Global Constraints

- Use only fake credentials and fake HTTP/provider services; perform no real external provider writes.
- Production resolver and persisted tenant Shell configuration must remain in the path.
- Do not strip or alter operator-provided Shell environment variables.
- Preserve the optional `RemoteClientFactory` nil path to production `buildClient`.

## Review Focus

- Wrong tenant/owner/service credential lookup must not satisfy Delivery dispatch; seed the same tenant and owner explicitly and assert the real stored token is used.
- The fake managed token must be absent from persisted/effective Shell configuration, provider create/exec requests, and command response; assert against the actual token read from storage.
- Test HTTP instrumentation must not mutate process-global client state; use the factory/client injection seam already available in the fixture.
- The test must exercise coexistence: persist the credential, resolve and dispatch Delivery, then resolve and execute the same tenant’s Shell before final boundary assertions.

---

### Task 1: Exercise the production managed credential path in the Shell boundary test

**Dependencies:** T55 Task 3 R1 (`65e02e15f..9bf28d5a037c00e7d898560e6a39b2a8f4034c11`).

**Owner role:** `backend_implementer`. **Validator role:** `backend_validator`. **Independent reviewer:** `reviewer`.

**Files:**
- Modify: `internal/application/repository/delivery_shell_isolation_http_test.go`
- Modify only the test fixture as needed: `internal/application/repository/delivery_recovery_http_test.go`

**Interfaces:**
- Consume `repository.NewMCPOAuthBindingStore(db)`, `CompleteBinding`, `LoadCredential`, and `appconnectorsvc.NewCredentialResolver`.
- Consume the existing fake GitHub HTTP service and Shell `RemoteClientFactory` seam.
- Produce one integration test whose Delivery dispatch retrieves the exact fake token from the persisted `MCPOAuthBindingStore` token row for tenant 1, owner `u1`, service `github`; Shell execution occurs for that same tenant and the managed token appears only in the captured outbound Delivery Authorization header.

- [ ] **Step 1: Add a failing persisted-credential boundary assertion.** Seed an active installation, owner membership, OAuth binding, and fake `types.MCPOAuthToken` with `MCPOAuthBindingStore.CompleteBinding`; update the connection to use `CredentialRefPrefix + "github"`; wire that store as the connection/credential source and use `appconnectorsvc.NewCredentialResolver` for dispatch. Assert the exact stored token is observed as Delivery Authorization while Shell observations exclude it.
- [ ] **Step 2: Run the focused test to establish RED.** Run `go test ./internal/application/repository/ -run '^TestManagedDeliveryCredentialNeverEntersGeneralShell$' -count=1 -v`. Expected: it fails before the production store/resolver wiring is completed, or at the new token/source assertion.
- [ ] **Step 3: Complete production resolver and test-scoped HTTP wiring.** Ensure the fixture sends Authorization capture through an injected `http.Client`/factory without assigning `http.DefaultClient`; preserve test isolation and existing fake-provider behavior.
- [ ] **Step 4: Run focused and adjacent checks.** Run the focused boundary test, `go test ./internal/modules/execution/sandbox/ -run 'TestWithWorkspaceEnvDefaults|TestResolveUsesOptionalRemoteClientFactoryAndDefaultsToProductionFactory' -count=1 -v`, and `git diff --check`. Expected: all pass; wrong tenant/owner lookup fails closed, actual stored token is observed only in Delivery Authorization, and operator Shell value remains intact.
- [ ] **Step 5: Commit and report.** Commit only the owned fixture/test changes. Record commit SHA, commands/results, diff hash, and report path in `.superpowers/sdd/plan-t55-integrated-review-repairs/task-3-managed-credential-repair-report.md`.

**Failure handling:** If existing HTTP client construction cannot inject the recorder without changing production behavior, stop and report the exact fixture seam needed; do not replace the production credential resolver with a test implementation.

## Plan self-review

- Finding coverage: the plan directly resolves `T55-T3-1`; it also removes the reviewer’s global-client isolation concern.
- Task boundary: this is one cohesive backend integration-test repair; no production behavior change is authorized by this finding.
- Interface consistency: store and resolver APIs are existing and already used by production composition; all provider and Shell test seams remain unchanged.
- Verification: the focused boundary and resolver regression commands are explicit; independent review and validation remain separate SDD steps.
