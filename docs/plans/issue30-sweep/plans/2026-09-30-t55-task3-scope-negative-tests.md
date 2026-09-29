# T55 Task 3 Credential Scope Regression Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]` syntax) syntax for tracking.

**Goal:** Pin tenant, owner, and service scoping in the production managed credential store with negative Delivery dispatch tests.

**Architecture:** Extend only the repository integration-test fixture from the Task 3 repair. For each mismatched token scope, keep the Delivery connection at `(tenant=1, owner=u1, service=github)`, seed a token under a different tenant, owner, or service via `MCPOAuthBindingStore.CompleteBinding`, and use `NewCredentialResolver(store)` for the dispatch attempt. Assert resolution fails closed before the GitHub stub sees Authorization or a provider write.

**Tech Stack:** Go, GORM/SQLite, Gin HTTP test harness, `MCPOAuthBindingStore`, production `CredentialResolver`.

**Spec:** `docs/plans/issue30-sweep/plans/2026-09-30-t55-task3-credential-path-repair.md`; finding `T55-T3-R2-1` in `.superpowers/sdd/plan-t55-integrated-review-repairs/task-3-managed-credential-repair-review.md`.

## Global Constraints

- Use fake credentials and local fake HTTP provider only.
- The production credential store and resolver remain in the tested path.
- No wrong-scope token may satisfy the selected tenant/owner/service connection.
- No source production behavior changes are authorized by this plan.

## Review Focus

- A token for the same owner/service in another tenant must not satisfy the connection.
- A token for another owner in the same tenant/service must not satisfy the connection.
- A token for the same tenant/owner under another service must not satisfy the connection.
- Each rejected dispatch must make zero outbound Authorization-bearing requests and zero provider writes.

---

### Task 1: Add production credential scope negative dispatch cases

**Dependencies:** Task 3 managed credential repair commit `944a097f006b22c28f4ff2dc5254f0a3fba15973`.

**Owner role:** `backend_implementer`. **Validator role:** `backend_validator`. **Independent reviewer:** `reviewer`.

**Files:**
- Modify: `internal/application/repository/delivery_shell_isolation_http_test.go`
- Modify only if fixture extraction is required: `internal/application/repository/delivery_recovery_http_test.go`

**Interfaces:**
- Consume the existing production `MCPOAuthBindingStore`, `CredentialResolver`, fake GitHub server, and Delivery dispatch HTTP fixture.
- Produce one table-driven test for mismatches `tenant`, `owner`, and `service`; each case maintains the selected connection `(1, u1, github)` but seeds no matching OAuth token row.

- [ ] **Step 1: Add three failing wrong-scope cases.** Build a normal production-credential fixture with the matching fake token and drive baseline, prepare, and approve for `conn-gh`. Immediately before dispatch, delete only the matching `(tenant=1, principal=u1, service=github)` token row and use `CompleteBinding` to persist one replacement token under a mismatched tenant, owner, or service tuple. Clear the Authorization recorder after prepare so dispatch-only observations are asserted. Dispatch must not reach `delivered`, must emit no Authorization-bearing request, and must produce zero additional fake-provider writes.
- [ ] **Step 2: Run the focused tests and establish the contract.** Run `go test ./internal/application/repository/ -run 'TestManagedCredentialWrongScopeCannotDispatch' -count=1 -v`. Expected: all three production store lookups fail closed after valid prepare/approval, before any dispatch Authorization or provider write. Because this adds regression coverage for an existing correct predicate, record the pre-change production-query evidence if the tests pass on first run; do not weaken production queries just to force a RED state.
- [ ] **Step 3: Keep the test on existing production interfaces.** Use `MCPOAuthBindingStore.CompleteBinding`, a test-only deletion of the matching token row after approval, and `NewCredentialResolver`; do not add a fake resolver or change production store queries. Use per-test SQLite database isolation and injected HTTP clients. The connection row remains `(tenant=1, owner=u1, credential_ref=mcp_oauth_token:github)` in every case.
- [ ] **Step 4: Run focused and adjacent verification.** Run both `TestManagedCredentialWrongScopeCannotDispatch` and `TestManagedDeliveryCredentialNeverEntersGeneralShell`, then `git diff --check`. Expected: all mismatched scopes fail before any outbound provider authorization/write; the positive isolation flow still passes.
- [ ] **Step 5: Commit and report.** Commit only the test/fixture edits and a report at `.superpowers/sdd/plan-t55-integrated-review-repairs/task-3-scope-negative-report.md`, recording commit, commands, outputs, and diff hash.

**Failure handling:** If the route reports a generic dispatch error but does not expose whether provider work occurred, inspect the fake GitHub stub’s write counters and captured requests directly. Do not infer zero writes solely from a failure response.

## Plan self-review

- Review finding coverage: the three independent scope dimensions and zero side effects are explicit.
- Task boundary: only an acceptance test extension is needed; no production change is justified by the finding.
- Verification: the positive isolation test remains in the repair suite and the negative tests assert external side effects directly.
