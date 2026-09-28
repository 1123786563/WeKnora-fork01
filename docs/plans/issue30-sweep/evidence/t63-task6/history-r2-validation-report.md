# T63 Task 6 History R2 — Independent Validation

- **Result:** PASS
- **Validated revision:** `5f121c96df241724428adb2187b9984caf333fb5`
- **Base revision:** `e033c95eaebbcfce2e257ae75a4e33510c75fb37`
- **Worktree:** `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep/.worktrees/issue30-b6-coordination/.worktrees/issue30-b6-t63`
- **Scope:** Plan `plan-t63-task6-history-r2.md`, Task 1 acceptance only.

## Checks

1. `go test ./internal/router/ -run 'TestLifecycleExitDeletesNothingAcrossGovernanceRows|TestLifecycleUnlistedVersusDeprecatedBehaviorDiffers|TestLifecycleRetireBlocksNewWorkEndToEnd' -count=1`
   - **Exit:** 0
   - **Output:** `ok  github.com/Tencent/WeKnora/internal/router  4.289s`
2. `git diff --check e033c95eaebbcfce2e257ae75a4e33510c75fb37..5f121c96df241724428adb2187b9984caf333fb5`
   - **Exit:** 0; no whitespace errors reported.
3. `git diff --name-only e033c95eaebbcfce2e257ae75a4e33510c75fb37..5f121c96df241724428adb2187b9984caf333fb5`
   - **Output:** `internal/router/routes_agent_marketplace_lifecycle_test.go`
4. `git status --short --branch`
   - **Output:** `## codex/issue30-t63` (clean worktree before this validation report was created).

## Assertion review

- The Release assertion compares `releaseAfter.AgentVersionID` with `release.AgentVersionID`, where `release` is from the tenant-scoped pre-exit `seededReleases` query. Each exact Release row is reloaded under tenant and ID scope.
- The Submission assertion compares `submissionAfter.AgentVersionID` with `submission.AgentVersionID`, where `submission` is from the tenant-scoped pre-exit `seededSubmissions` query. Each exact Submission row is reloaded under tenant and ID scope.
- The independent comparisons catch a lifecycle rewrite that changes both links consistently. The existing Release-to-Submission equality check remains in place as a separate relationship assertion.
- Existing row-count, Adoption, Variant pin, manifest License, run/request/artifact provenance, and related history checks remain in the test.

## Acceptance and limits

Task 1 acceptance is met: both source-version links are asserted against their respective pre-exit values; all three requested targeted router tests pass; the scoped diff check is clean. No API/authentication, migration, cancellation, or error-handling behavior was changed by this test-only revision; those areas are outside this task's acceptance scope. No broader test suite was run.
