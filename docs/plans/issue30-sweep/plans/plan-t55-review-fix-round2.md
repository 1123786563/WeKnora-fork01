# T55 Review Fix Round 2 — unknown recovery evidence and lease error fencing

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Close the remaining accepted medium findings from the first Task2/Task5 independent reviews with two isolated test/code changes.

**Architecture:** Task 1 strengthens only the T25 HTTP wire test: record every provider write method/path at stub ingress, prove resolve produces no write request, prove subsequent recovery dispatch adds only PR creation, and assert all credential-test response states. Task 2 strengthens only the mobile-core recovery module and its test: if a read or write rejects after the scope lease is revoked, translate the late failure to `DELIVERY_SCOPE_CHANGED` before exposing the backend or conflict error.

**Tech Stack:** Go `httptest`/testify; TypeScript node:test/tsx and mobile typecheck.

**Spec:** Issue #55 snapshot `docs/plans/issue30-sweep/issues/issue-55.md`; original implementation `docs/plans/issue30-sweep/plans/plan-t55.md` Tasks 2 and 5; accepted review evidence in `docs/plans/issue30-sweep/T55-review-fix-ruling.md`, Task2 fix commit `d83ff539b4e96358cd43c9c603cb076d97d539ab`, Task5 commit `a07ac18da9c8432b6cd202c344c52a49f5ca1823`.

## Global Constraints

- No production Go behavior changes. Task 1 owns only `internal/application/repository/delivery_recovery_http_test.go` in T55 worktree; Task 2 owns only `delivery-recovery.ts` and its test in the T55 Task5 worktree.
- Do not mix worktrees or shared output files; reports are distinct.
- Keep test-only fake credentials. Never enable live GitHub writes or use real credentials.
- Preserve precise boundary statement: the Go fixture uses real handler/service/A03 flow and local route mounts with injected identity; it does not exercise production router auth middleware.
- Each task commits locally after its own focused checks. An independent reviewer assesses each completed diff.

## Review Focus

1. The stub records all provider requests using POST/PATCH/PUT/DELETE at request ingress, regardless of whether the path is recognized. Resolve must leave this complete write trace unchanged.
2. After `unknown` resolves from branch facts to `pushed`, the next dispatch must change only the PR creation count; Git blob/tree/commit/ref write counts remain unchanged. The resulting delivery is `delivered`.
3. Credential test verifies baseline, prepare, approval, dispatch, resolve, retry, delivery GET, and final GET HTTP statuses. Resolve response state is `pushed`, retry response state is `delivered`, final GET is successful and returns delivered.
4. If either `remote.delivery` or a recovery write rejects after the captured `ScopeLease` becomes inactive, `createDeliveryRecovery` returns `DeliveryRecoveryError('DELIVERY_SCOPE_CHANGED')`; active-lease backend and 409 behavior remains unchanged.

---

### Task 1: Complete unknown-recovery write trace and response-state evidence

**Dependencies:** T55 Task2 repair commit `d83ff539b4e96358cd43c9c603cb076d97d539ab`; current review identifies two MEDIUM and one LOW gap.

**Role:** `backend_implementer`; independent reviewer `reviewer`; no production validator task needed because this is test-only and focused test evidence is required.

**Worktree:** existing `codex/issue30-t55` at the reviewed repair commit. Do not touch the Task5 branch.

**Owned file:** `internal/application/repository/delivery_recovery_http_test.go` only.

**Report:** `.superpowers/sdd/plan-t55-review-fix-round2/task-1-report.md`.

**Step 1 — Record every external write request.** In `recoveryGitHubStub.serve`, append method plus sanitized request path for every POST, PATCH, PUT or DELETE before route dispatch, protected by the existing mutex. Keep existing per-endpoint counters. Provide a defensive-copy snapshot helper.

**Step 2 — Prove unknown resolution is read-only and recovery is PR-only.** In `TestT25UnknownResolvesFromRemoteFactsOverHTTP`, compare the full write-request trace before/after `/resolve` and require no change. Before the subsequent dispatch snapshot the full trace and endpoint counters; afterward require exactly one new `POST /repos/octocat/hello/pulls` and no changes to any Git blob/tree/commit/ref writes. Assert response status/state and final delivered record. This path-specific proof supplements, rather than substitutes for, the partial-PR-failure test.

**Step 3 — Make credential response scans non-vacuous.** Assert each captured resolve/retry/final GET response status is the expected success code and decode the envelopes to assert `pushed` for resolve and `delivered` for retry/final GET before including bodies in the credential scan. Preserve all existing response, action-row, delivery-row and workspace checks.

**Step 4 — Verify and commit.** Run `go test ./internal/application/repository/ -run 'TestT25' -count=1`, `go test ./internal/application/repository/ -run 'TestDeliveryCollaboration|TestT25' -count=1`, and `git diff --check`. Commit only the owned test file as `test(codedelivery): complete unknown recovery write trace assertions`.

**Acceptance:** Unrecognized write endpoints cannot escape the resolve no-write check; post-resolve dispatch is proven to create only the PR; credential scan is gated on expected successful states; no production files change.

**Failure handling:** If the real behavior emits a non-PR write or status/state differs, retain the failing test and report it as a production blocker; do not alter production code in this task.

---

### Task 2: Drop late read/write errors after lease revocation

**Dependencies:** Task5 commit `a07ac18da9c8432b6cd202c344c52a49f5ca1823`; Task5 reviewer finding that rejected late responses bypass scope fencing.

**Role:** `frontend_implementer`; independent reviewer `reviewer`; targeted local verification in task.

**Worktree:** existing `codex/issue30-t55-t5` worktree at Task5 implementation checkpoint. Do not touch the Go task worktree.

**Owned files:**
- `packages/mobile-core/src/delivery/delivery-recovery.ts`
- `packages/mobile-core/src/delivery/delivery-recovery.test.ts`

**Report:** `.superpowers/sdd/plan-t55-review-fix-round2/task-2-report.md`.

**Step 1 — Add deterministic read-failure race test.** A fake `remote.delivery` revokes the same lease and then rejects. Assert the operation rejects with `DeliveryRecoveryError` code `DELIVERY_SCOPE_CHANGED`, not backend.

**Step 2 — Add deterministic write-failure race test.** A fake remote first returns a `pushed` record, then `dispatchDelivery` revokes the same lease and rejects (including a 409-shaped error case if straightforward). Assert `DELIVERY_SCOPE_CHANGED` takes precedence over `DELIVERY_STATE_CONFLICT`/backend.

**Step 3 — Fix catch paths only.** In each relevant read/write catch, check `leaseActive(lease)` before translating the original error. If inactive, throw `DeliveryRecoveryError('DELIVERY_SCOPE_CHANGED', ...)`; otherwise preserve existing 409/backend translations. Keep post-success lease recheck and all state routing unchanged.

**Step 4 — Verify and commit.** Run `pnpm exec tsx --test packages/mobile-core/src/delivery/delivery-recovery.test.ts`, `pnpm --filter @weknora/mobile typecheck`, and `git diff --check`. Commit only the two owned files as `fix(mobile-core): discard recovery errors after scope revocation`.

**Acceptance:** Both deterministic rejected-request races map to scope change; active lease backend and 409 tests still pass; typecheck passes; only the two owned files change.

**Failure handling:** If lease revocation races are not deterministic, test by revoking and rejecting within the awaited remote method itself; do not use sleeps. If the fix changes active-scope behavior, retain a failing test and report.

---

## Interface and Scope Preflight

- Task 1 Go fixture and Task 2 TypeScript module own disjoint worktrees and files; no interface dependency exists between them.
- Task 1 consumes the stub's synchronized call trace and existing JSON envelopes only; Task 2 consumes existing `leaseActive` semantics and `DeliveryRecoveryError` API only.
- No Marketplace files or shared database/port/artifact resources are touched.
- Both tasks have concrete focused validation commands and exact report paths.
