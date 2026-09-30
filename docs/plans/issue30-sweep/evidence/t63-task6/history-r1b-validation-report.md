# T63 Task 6 History Evidence R1 — Validation Report

- **Revision under validation:** `e033c95eaebbcfce2e257ae75a4e33510c75fb37`
- **BASE:** `707b0f803dfd3c7cf31271de1b1acdda5dab1fe7`
- **Plan:** `docs/plans/issue30-sweep/plans/plan-t63-task6-history-r1.md`
- **Scope:** Only the plan-owned lifecycle router test was reviewed; no source/test files were changed by validation.
- **Overall:** **PASS** for the assigned R1 acceptance checks.

## Commands and results

1. `git rev-parse HEAD`
   - Output: `e033c95eaebbcfce2e257ae75a4e33510c75fb37` (matches assigned validation HEAD).
2. `git status --short`
   - Output: empty (worktree clean before writing this report).
3. `go test ./internal/router/ -run 'TestLifecycleExitDeletesNothingAcrossGovernanceRows|TestLifecycleUnlistedVersusDeprecatedBehaviorDiffers|TestLifecycleRetireBlocksNewWorkEndToEnd' -count=1`
   - Output: `ok   github.com/Tencent/WeKnora/internal/router 5.360s`
   - **PASS.** This executes the requested real-router lifecycle tests against the checked-out revision.
4. `git diff --check 707b0f803dfd3c7cf31271de1b1acdda5dab1fe7..e033c95eaebbcfce2e257ae75a4e33510c75fb37`
   - Output: empty; exit status 0.
   - **PASS.**
5. `git diff --unified=8 707b0f803dfd3c7cf31271de1b1acdda5dab1fe7..e033c95eaebbcfce2e257ae75a4e33510c75fb37 -- internal/router/routes_agent_marketplace_lifecycle_test.go`
   - **PASS.** The R1 delta is limited to the planned test file and adds exact link assertions.

## Acceptance evidence

- Adoption is loaded with `(tenant_id, id)` before exits and reloaded with the same key after the four lifecycle exits. The test asserts preserved `ListingID`, `AcceptedReleaseID`, and expected `State == "ended"`.
- The exact Variant is reloaded with `(tenant_id, id)`. Its `AdoptionID`, pinned `ReleaseID`, `LocalAgentID`, and `LocalAgentVersionID` are checked; its pinned release is constrained to the seeded release IDs.
- For each retained Release, the test reloads its exact Submission and source AgentVersion using tenant-scoped ID predicates, asserting the Submission's version and source Agent identity agree with the Release links.
- The parsed historical Release manifest's `license_id` is compared to the seeded License ID (`MIT`), and the exact License row is reloaded and its name compared to the seeded registry value. Thus Adoption / Variant pin / Release-Version / manifest-license links are independently asserted by test code and exercised by the passing targeted run.
- Existing governance row counts and lifecycle response assertions remain in the test around the new checks.

## Limitations / risks

- The fixture uses original Releases, not Fork Releases. Fork-source lineage is outside this Task 6 acceptance and remains covered by the dedicated lineage route tests per the plan.
- Provenance rows are asserted after the sequence of all four exits, not after each individual exit in isolation. The test does prove that the exact links survive the complete sequence; it does not localize which exit could mutate a link if a future regression were introduced.
- No broader package, migration, authentication, or authorization suite was run because the assignment limits validation to the two specified commands and owned test scope. The targeted test does use the real router and tenant-scoped DB reads.
