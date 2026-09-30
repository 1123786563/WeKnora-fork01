# T08 Active-Membership Workbench Fixture Fix Plan

> **For Codex:** Execute with SDD exact uncommitted checkpoint and independent narrow Review. No commit.

**Goal:** Restore the four tenant-scoped repository tests that now fail admission because their migrated SQLite fixtures create a user/session but omit the active tenant membership required by reviewed T08 actor admission.

**Sources:** `t08-actor-membership-fix-review.md`, T19 budget-pause Task1 full repository test log and its independent review; `workbench_http_integration_test.go:openWorkbenchHTTPDB`, `agent_run_driver_test.go:TestWorkbenchOwnedRunIsTenantScoped`, `artifact_version_test.go:TestArtifactVersionCrossTenantIsolation`. The fifth failure, `TestCancelRunReleasesSlotAndDeleteFences`, is a distinct lifecycle-event behavior and outside this fixture task.

**Global Constraints:** Test fixture only. Do not weaken the production active-member predicate or alter Workbench behavior/assertions. Preserve user/main checkout. This is a narrow mechanical edit in shared integration Worktree after the full repository failures are recorded.

**Review Focus:** insert an active membership for each legitimate fixture actor/tenant using the migrated schema and valid role/status, without granting foreign users/tenants access. All four formerly conflicting admission tests and the focused admission tests must pass.

## Task 1 — add authoritative fixture membership

**Depends on:** T08 actor membership fix reviewed PASS. **Role:** mechanical_worker. **Owned files:** `internal/application/repository/workbench_http_integration_test.go`, `agent_run_driver_test.go`, `artifact_version_test.go`, report/checkpoint only.

1. RED: reproduce the four `agent runtime conflict` failures against the reviewed actor predicate; retain exact log. Keep the separate lifecycle event-count failure out of this task.
2. GREEN: add an active `tenant_members` row in `openWorkbenchHTTPDB` for tenant 1/u1 and in the two tenant-2 fixtures for tenant 2/u2 after each user row and before admission. Keep all product assertions identical.
3. Run all four target tests, focused actor admission tests, `git diff --check`; exact before/after source hash checkpoint and independent narrow review. Full repository package rerun is central after concurrent agents finish.

**Failure handling:** If the fixture schema imposes another required field, satisfy the migrated table contract in this fixture only; do not bypass membership validation.
