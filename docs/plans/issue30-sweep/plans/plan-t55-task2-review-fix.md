# T55 Task 2 Review Findings Fix Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the accepted #55 Task 2 and Task 7 evidence findings plus the independent #55 Task 3 env-whitelist test finding through three disjoint, independently reviewable test-only tasks.

**Architecture:** Task 1 repairs the existing `delivery_recovery_http_test.go`, preserving real SQLite, handlers, A03 approval and the GitHub wire stub while adding remote-read, route-boundary and credential-surface evidence. Task 2 independently repairs the existing `workspace_env_test.go` whitelist assertion by comparing against an immutable pre-call key set, since `withWorkspaceEnvDefaults` mutates the supplied map. Task 3 independently strengthens the opt-in real GitHub provider test with draft-state, no-write-on-rejection and safe-baseline assertions. Each task owns one test file, a separate worktree and a distinct report/review checkpoint.

**Tech Stack:** Go, Gin, `httptest`, SQLite/GORM, testify.

**Spec:** `docs/plans/issue30-sweep/issues/issue-55.md`; original implementation plan `docs/plans/issue30-sweep/plans/plan-t55.md` Tasks 2, 3 and 7; Task 2 review checkpoint `82505197e150074b89447452a692252c3b3fd93c` and report `.superpowers/sdd/plan-t55/task-2-report.md`; Task 3 checkpoint `90580a2fa171a50fb165e32033cb6ecff75207fc` and report `.superpowers/sdd/plan-t55/task-3-report.md`; Task 7 report `.superpowers/sdd/plan-t55/task-7-report.md`.

## Global Constraints

- Keep production behavior and production source unchanged; Task 1 owns `internal/application/repository/delivery_recovery_http_test.go`, Task 2 owns `internal/modules/execution/sandbox/workspace_env_test.go`, and each task owns its review-fix documentation/report.
- Preserve the three existing T25 tests and their externally observable acceptance behavior.
- Never use a usable provider credential; keep `t25ProbeToken` as a fake test value.
- Keep the full migrated SQLite fixture, real delivery/A03 handlers and the GitHub HTTP wire stub.
- Do not describe context injection as end-to-end authentication. State exactly which production route registration/gates are exercised and which identity/authentication boundary is supplied by a test adapter.
- Run each task in its own worktree; no shared file writes or shared worktree coordination.
- Task 1 owner role is `backend_implementer`; Task 2 owner role is `mechanical_worker`; Task 3 owner role is `backend_implementer`. Each gets an independent reviewer after its own diff checkpoint.

## Review Focus

1. **Unknown resolution must use remote facts:** capture the provider-call snapshot before and after `ResolveDeliveryUnknown`; assert it reads the exact branch ref and filtered PR list, asserts the branch commit is the expected commit, asserts the PR list is empty, and asserts that resolution emits no POST/PATCH. Then preserve the existing PR-only dispatch recovery proof.
2. **Production route registration and authorization boundary:** the route researcher confirmed that the repository test cannot directly initialize `RegisterWorkbenchDeliveryRoutes` here: `rbacGuards` and its setup helpers are unexported, full `NewRouter` construction requires many unrelated services, and production authentication requires broader DB-backed fixtures. Keep this test's local handler mounts, including the resolve endpoint, but describe it precisely as real handler/service HTTP with injected tenant/user/role identity. Cite `routes_workbench.go` and `routes_app_connectors.go` for production path/registration declarations, plus existing router evidence where relevant. Do not claim production router middleware or credential authentication is exercised. Record this accepted boundary and residual coverage risk in the report.
3. **Credential data surface completeness:** capture baseline, prepare, approve, initial dispatch, retry dispatch, delivery read, resolve and relevant post-resolve response bodies; assert no probe token in those bodies, every persisted `app_actions` field/value associated with the delivery, and every persisted delivery string column. Require all expected dispatch, approval and resolve steps to succeed so the negative checks cannot pass vacuously.

---

### Task 1: Repair T25 HTTP recovery evidence

**Dependencies:** Original #55 Task 2 implementation checkpoint `82505197e150074b89447452a692252c3b3fd93c` and its passing targeted tests. No production task dependency.

**Role:** `backend_implementer`; independent review by `reviewer`; behavior verification by `backend_validator` after implementation.

**Files:**
- Modify: `internal/application/repository/delivery_recovery_http_test.go` only.
- Report: `.superpowers/sdd/plan-t55-review-fix/task-1-report.md`.

**Interfaces:**
- Consumes: existing test fixture `newRecoveryEnv`, `recoveryGitHubStub`, `seedApprovedDelivery`, `dispatchState`; production `router.RegisterWorkbenchDeliveryRoutes`, `router.RegisterAppConnectorRoutes`; current action and delivery persistence models.
- Produces: the three existing T25 HTTP tests, with explicit proof for resolver remote-read behavior, route registration boundary and credential containment.

**Step 1: Inspect exact production route declarations and test fixtures.**

Read `internal/router/routes_workbench.go` (`RegisterWorkbenchDeliveryRoutes`), `internal/router/routes_app_connectors.go` (`RegisterAppConnectorRoutes`), guard constructors, `internal/handler/app_action.go` and action persistence. The route researcher has already confirmed that production registration is impractical within the one-file repository-test boundary because router guard setup is unexported and full app/auth construction expands into unrelated services and DB fixtures. Do not spend time attempting to reconstruct the entire app router. Do not edit files during this step.

Expected evidence: test requests continue through the real handlers and service graph on the actual route paths, with tenant/user/role context injected by the fixture. Test comments and report accurately identify this as handler/service HTTP evidence, not production route registration or auth middleware evidence. Cite production route declarations and the separate router test evidence, and explicitly state they are not exercised by this test.

**Step 2: Add remote-fact trace and snapshots to the GitHub stub.**

Extend `recoveryGitHubStub` only as needed to record `GET` methods, full request paths and query values; make branch-ref responses return the exact commit SHA currently on the simulated branch. Keep synchronized access under `s.mu`. In `TestT25UnknownResolvesFromRemoteFactsOverHTTP`, snapshot calls before and after resolve, then assert:

```go
beforeResolve := env.github.snapshotCalls()
w := env.do(t, "POST", "/api/v1/workbench/executions/r1/delivery/"+deliveryID+"/resolve", "")
afterResolve := env.github.snapshotCalls()
require.Equal(t, beforeResolve["GET /repos/octocat/hello/git/ref/heads/weknora/task/s1"], afterResolve["GET /repos/octocat/hello/git/ref/heads/weknora/task/s1"]-1)
require.Equal(t, beforeResolve["GET /repos/octocat/hello/pulls?head=octocat:weknora/task/s1"], afterResolve["GET /repos/octocat/hello/pulls?head=octocat:weknora/task/s1"]-1)
```

Use the stub's actual escaped path/query representation when choosing counter keys. Additionally assert the branch read returns the recorded pushed commit, the filtered PR query returns zero matches, and every `POST /...` or `PATCH /...` counter is unchanged across resolve. Keep the final `pushed` state assertion and prove that the subsequent dispatch increments only PR creation, leaving all Git object/ref write counts unchanged.

**Step 3: Precisely label handler HTTP and authentication evidence.**

Keep the local Gin registrations in `newRecoveryEnv`, since constructing the production router exceeds this test's bounded fixture and ownership. Ensure all tested paths match the production declarations, including `POST /workbench/executions/:run_id/delivery/:delivery_id/resolve`, and ensure the credential test actually invokes the resolve route. Update introductory comments to say: “real handler/service HTTP with a real migrated SQLite database and test-injected principal context; local route mounts mirror production paths declared in `router/routes_workbench.go` and `router/routes_app_connectors.go`; production router/auth middleware is not exercised.” Cite those route declarations in the comment. Do not add a misleading synthetic gate or claim real authentication. Existing persisted A03 digest/fence approval remains in scope as handler/service evidence.

**Step 4: Capture the entire credential-response and persistence surface.**

In `seedApprovedDelivery`, retain and return the HTTP response bodies for baseline, prepare and approval alongside the delivery ID, or provide an output collector owned by the fixture. In the credential test, capture those responses plus the initial dispatch, retry, delivery read, resolve and final read. Assert expected status codes before scanning bodies. Query the action row(s) for the delivery `ActionID`; serialize every persisted field (including input, result/output, error, risk, digest, state, tenant/actor identifiers and timestamps) to JSON and assert the probe is absent. Retain the delivery row scan across all string fields. Keep the workspace walk. Assert the fake probe appears in the stub's `Authorization` observations and that the requests reached expected provider endpoints; a request count without positive header observation is insufficient.

**Step 5: Run focused verification and record exact output.**

Run from repository root:

```bash
go test ./internal/application/repository/ -run 'TestT25' -count=1
go test ./internal/application/repository/ -run 'TestDeliveryCollaboration|TestT25' -count=1
git diff --check
```

Expected: all focused and adjacent tests pass; `git diff --check` reports no whitespace errors. Report any RED observation accurately; the original Task 2 report records that its first authored run passed, so do not invent a RED result. Record route/auth boundary, request trace assertions, persisted fields inspected and command outputs in the task report.

**Step 6: Commit the test-only review fix.**

Commit only `internal/application/repository/delivery_recovery_http_test.go` with message:

```text
test(codedelivery): strengthen T25 unknown and credential boundary evidence
```

Before commit, confirm no production source, generated file or unrelated worktree changes are staged.

**Acceptance:**

- Remote resolution has asserted branch ref and PR-list reads; it asserts branch SHA and empty matching PR set, and proves zero remote writes during resolution.
- Recovery after unknown creates the PR without repeating blob/tree/commit/ref pushes.
- HTTP requests use real handlers/service with injected principal context and local route-equivalent mounts; the test and report explicitly state production route registration/auth middleware are outside this fixture, citing their source declarations.
- Credential probe is positively observed only in provider Authorization headers and absent from every captured HTTP response, the workspace, all relevant `app_actions` persisted values and all delivery string columns.
- The three T25 tests and adjacent delivery collaboration regression suite pass.

**Failure handling:** Do not expand this one-file task into full `NewRouter` wiring or edit production router/auth code. The route researcher already established that those constructors require broad unrelated wiring and unexported guard setup. If an assertion reveals a real production leak or behavior failure, keep it failing, record it as a blocker, and route a separate scoped production repair through SDD. Report production-router/auth coverage as a residual limitation.

### Task 2: Make the sandbox env whitelist test compare against an immutable input key set

**Dependencies:** Original #55 Task 3 checkpoint `90580a2fa171a50fb165e32033cb6ecff75207fc`; independent reviewer finding that `withWorkspaceEnvDefaults` mutates its `env` argument, invalidating the current test's post-call comparison against `input`.

**Role:** `mechanical_worker`; independent review by `reviewer`; targeted behavior verification by `backend_validator`.

**Separate worktree:** `codex/issue30-t55-t3-review-fix`, based on the Task 3 reviewed checkpoint above. Do not use Task 1's worktree.

**Files:**
- Modify: `internal/modules/execution/sandbox/workspace_env_test.go` only.
- Report: `.superpowers/sdd/plan-t55-review-fix/task-2-report.md`.

**Interfaces:**
- Consumes: `withWorkspaceEnvDefaults(map[string]string) map[string]string`, which returns the same map and fills unset workspace keys.
- Produces: a regression test that detects any newly inserted key outside the approved workspace-path key set.

**Step 1: Preserve a pre-call snapshot of configured keys.**

In `TestWithWorkspaceEnvDefaultsPreservesConfiguredValuesAndAddsNothingElse`, copy the input before calling the mutating helper:

```go
configuredKeys := make(map[string]struct{}, len(input))
for key := range input {
    configuredKeys[key] = struct{}{}
}
got := withWorkspaceEnvDefaults(input)
```

Use `configuredKeys`, never the mutated `input`, to classify added keys. Assert configured PATH and flag values are preserved, the customized output directory remains unchanged, and the missing input directory is defaulted.

**Step 2: Assert the exact allowed added-key set.**

Collect every key in `got` absent from `configuredKeys`, then assert the set equals exactly `{sessionInputEnvVar}` for this fixture. Also assert `skillOutputEnvVar` was not added because it was configured. Keep the nil-input test asserting the exact two workspace path keys. This detects any future extra environment injection while respecting the helper's intentional in-place mutation contract.

**Step 3: Run targeted verification and hygiene.**

Run from this task's worktree root:

```bash
go test ./internal/modules/execution/sandbox/ -run 'TestWithWorkspaceEnvDefaults' -count=1
git diff --check
```

Expected: both whitelist tests pass and `git diff --check` reports no whitespace errors. Report exact output and confirm only the owned test file changed.

**Step 4: Commit the assertion-only fix.**

Commit only `internal/modules/execution/sandbox/workspace_env_test.go` with message:

```text
test(sandbox): snapshot configured keys before workspace env defaults
```

**Acceptance:**

- The test classifies additions against a snapshot taken before the mutating function call.
- The non-nil fixture passes only when the newly added key set is exactly the one missing approved workspace key.
- The custom configured workspace output key and unrelated configured values are preserved.
- The nil-input fixture still asserts exactly the approved pair of workspace path keys.
- The targeted test passes; only the owned test file is changed.

**Failure handling:** If the test reveals the production helper adds an extra key, keep the failing assertion and report the newly observed key; do not edit `session_manager.go` in this task. A production change requires a separately scoped SDD task.

### Task 3: Strengthen the opt-in real GitHub duplicate-dispatch loop

**Dependencies:** Original #55 Task 7 checkpoint/implementation on branch `codex/issue30-t55-t7`; independent Task 7 reviewer confirmed three medium findings: draft state is not directly asserted on initial and repeated reads; unchanged remote SHA does not prove the rejected retry sent no write request; and the test stages only README while delivery diff calculation interprets every omitted baseline file as deletion.

**Role:** `backend_implementer`; independent review by `reviewer`; targeted behavior validation by `backend_validator`.

**Separate worktree:** reuse the isolated `codex/issue30-t55-t7` worktree, based on the Task 7 implementation checkpoint. Do not use Task 1 or Task 2 worktrees.

**Files:**
- Modify: `internal/modules/codedelivery/github_real_test.go` only.
- Report: `.superpowers/sdd/plan-t55-review-fix/task-3-report.md`.

**Interfaces:**
- Consumes: existing opt-in environment keys `WEKNORA_GITHUB_TEST_TOKEN`, `WEKNORA_GITHUB_TEST_REPO`, `WEKNORA_GITHUB_TEST_WRITABLE=1`; `GitHubClient.Tree`, `PullRequestForHead`, `GitHubAPIBaseURL`, and `NewGitHubClientFactory`.
- Produces: opt-in test `TestGitHubRealRecoveryLoopNoRepeatPush` that verifies the initial and repeated real PR receipts are draft, the rejected duplicate makes no HTTP write request, and the test cannot accidentally delete baseline files from the remote branch.

**Step 1: Add a safe baseline preflight before any writes.**

After obtaining `baseline` and before preparing/approving/dispatching, call `client.Tree(ctx, baseline)` and assert its file map contains exactly one entry, `README.md`. Keep the fixture workspace limited to README only. If the dedicated writable repository has any other baseline file, fail closed before delivery preparation or any remote write and report the expected repository precondition. This ensures `DiffAgainstBaseline` cannot treat LICENSE, `.gitignore` or any other omitted file as a deletion. Do not weaken the precondition or let the test proceed against a non-empty baseline.

**Step 2: Assert draft PR status on initial and repeat reads.**

After first dispatch, use `client.PullRequestForHead(ctx, view.Branch, defaultBranch)` (or the exact existing client method signature) and assert a non-nil receipt, the expected PR number/URL, and `Draft == true`. After the rejected duplicate, query the PR again and assert the same PR number and URL and `Draft == true`; keep the delivered view assertions. These checks prove the real provider behavior matches the requirement and that repeat reads did not change the PR state.

**Step 3: Count outbound real-provider writes around the duplicate.**

Wrap the real `http.DefaultTransport` in a task-local counting `http.RoundTripper` and pass an `http.Client{Transport: counter}` into `NewGitHubClientFactory`. Snapshot write-method requests immediately before the duplicate `DispatchDelivery`, then snapshot again after it returns `ErrDeliveryState`. Assert there was no increment for any write method (`POST`, `PATCH`, `PUT`, or `DELETE`); GET polling used for assertions is permitted. Record method and sanitized path only—never log Authorization values or the test token. Continue asserting branch head and PR identities remain unchanged.

**Step 4: Verify blocked-env and local behavior without external writes.**

With opt-in variables absent, run the specific real-provider test and require its expected `blocked-env` skip. Run the codedelivery package tests and `git diff --check`. Do not supply fake credentials or turn on `WEKNORA_GITHUB_TEST_WRITABLE`; no real-provider write should be made during this repair. The earlier Task 7 report already records the opt-in test's blocked-env result and package test evidence, but rerun after source edits.

```bash
go test ./internal/modules/codedelivery/ -run 'TestGitHubRealRecoveryLoopNoRepeatPush' -count=1 -v
go test ./internal/modules/codedelivery/ -count=1
git diff --check
```

**Step 5: Commit the assertion-only test changes.**

Commit only `internal/modules/codedelivery/github_real_test.go` with message:

```text
test(codedelivery): assert safe real-provider recovery boundary
```

**Acceptance:**

- The live remote baseline is proven to contain only the file mirrored by the fixture before any delivery writes are attempted.
- Initial and repeated real-provider PR reads assert `Draft == true` and stable PR identity.
- The duplicate dispatch returns `ErrDeliveryState` and an HTTP transport counter proves it emitted zero POST/PATCH/PUT/DELETE requests.
- No credentials or external writes are used during ordinary verification; absent opt-in remains a truthful blocked-env skip.
- Only the owned integration test file changes, and its task report includes exact commands/output and the safe baseline precondition.

**Failure handling:** If the remote baseline has any additional file, fail closed before approval/dispatch and report the repository setup requirement; do not proceed and risk a deletion. If the public API lacks the expected receipt field, inspect the interface and use its equivalent observable draft field without weakening the assertion. If an assertion indicates production duplicate dispatch sends a write, leave the test failing and open a separate production-fix SDD task.

---

## Interface and Scope Preflight

- Task 1 sole code file owner: `internal/application/repository/delivery_recovery_http_test.go`.
- Task 2 sole code file owner: `internal/modules/execution/sandbox/workspace_env_test.go`.
- Task 3 sole code file owner: `internal/modules/codedelivery/github_real_test.go`.
- Shared plan/report metadata has one coordinator owner; task reports are distinct.
- No production files, migrations, schemas, generated assets or other tasks' tests are writable under this repair.
- No interfaces are consumed from concurrent #63/#64 Marketplace tasks; integration order does not affect this test-only repair.
- Original AC mapping retained: #55 AC1 → remote-resolution and exactly-once assertions; AC3 → production route/handler boundary labeling; credential-isolation requirement → full HTTP/workspace/persistence probes.

## Review Ruling

Task 1 accepts all three Task 2 reviewer observations. The two medium findings identify missing direct evidence: resolver calls are not inspected, and the fixture directly mounts handlers while claiming production route/auth coverage. The low finding is valid because the probe scan omits setup/approval responses and persisted A03 action data. The bounded fix adds those assertions without changing production behavior. The route researcher confirmed production registration is impractical within the one-file repository-test boundary: guards are unexported, `NewRouter` requires many unrelated services, and auth requires broader DB-backed setup. Ruling: keep route-equivalent mounts and injected principal context, accurately label the real handler/service HTTP boundary, cite production route declarations, and preserve the untested production route/auth middleware gap as a residual risk.

Task 2 accepts the independent Task 3 finding. Because `withWorkspaceEnvDefaults` mutates its input map, iterating over `input` after invocation treats every inserted key as configured and skips the whitelist assertion. The fix snapshots the key set before invocation; it does not change production code.

Task 3 accepts the three independent findings on original Task 7. The test must assert the actual remote PR is draft both before and after rejected duplicate dispatch; immutable branch/PR identity alone does not prove zero writes, so a counting transport must establish that; and a README-only workspace against a broader remote tree would make the delivery diff report every omitted baseline file as deleted. The test will fail closed unless the remote baseline contains only README, before it attempts writes. The stricter dedicated-repository precondition may make some opt-in runs fail early, but prevents accidental deletion and is appropriate for a test explicitly documented to use a dedicated repository.

**Risk if this ruling is wrong:** If a reviewer concern is only wording, extra evidence adds some fixture complexity; if the missing evidence is material, leaving it would allow remote state or credential leaks to pass without detection. Route setup could couple this test to broader app auth wiring; the fallback preserves a truthful and maintainable scope while identifying the evidence still missing.
