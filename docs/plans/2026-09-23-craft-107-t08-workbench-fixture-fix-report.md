# T08 Active-Membership Workbench Fixture Fix Report

**Status:** DONE — fixture-only corrections; production behavior and assertions are unchanged.

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`
- Before SHA-256 — `workbench_http_integration_test.go`: `bc4a9016cf39e3df914cd6fe95caf7a74c0bb29daabaf5028f81c7cad72f1242`
- After SHA-256 — `workbench_http_integration_test.go`: `c3da63d5c19a782bcfe097382792a0ece6fc33d4878202b4e01e2f581f8a6865`
- Before SHA-256 — `agent_run_driver_test.go`: `0c769dc36bcd18c38ebd8b7c689098cf5fd283e3943651be6272febe1a1dfd6d`
- After SHA-256 — `agent_run_driver_test.go`: `3c2edb14a356734bd0c25acb171da4bfe4174f02e8999b8bee961e9a6831c24f`
- Before SHA-256 — `artifact_version_test.go`: `a75cccb774ce93d6a39bd2633655aeb7e35c63072a4d73649dafcc3995bdf5c5`
- After SHA-256 — `artifact_version_test.go`: `a1eaf1545df6069f5236ba9eed9f4166a61b7acac8b7275e074f743e34814cc0`

No production file, assertion, separate lifecycle event-count failure, or unrelated fixture was changed. No commit was created.

## RED → GREEN evidence

- RED command:
  `go test ./internal/application/repository -run '^(TestWorkbenchHTTPOwnershipUsesRealStoreAndProjection|TestWorkbenchHTTPSourceEventToSnapshotAndSSEUsesRealRepository|TestWorkbenchOwnedRunIsTenantScoped|TestArtifactVersionCrossTenantIsolation)$' -count=1`
- RED result: all four tests failed at `Admit` with `agent runtime conflict` because their actors lacked active tenant membership.
- GREEN: seeded tenant 1/u1 as `admin`/`active` in shared `openWorkbenchHTTPDB`, and tenant 2/u2 as `admin`/`active` in each tenant-2 fixture before admission.
- Focused target plus six actor-admission tests:
  `go test ./internal/application/repository -run '^(TestWorkbenchHTTPOwnershipUsesRealStoreAndProjection|TestWorkbenchHTTPSourceEventToSnapshotAndSSEUsesRealRepository|TestWorkbenchOwnedRunIsTenantScoped|TestArtifactVersionCrossTenantIsolation|TestAdmissionUsesActiveMembershipInsteadOfHomeTenant|TestCraftRunAdmissionRejectsInactiveActorMembershipAndUser|TestCraftRunAdmissionStoresOwnerAndAuthenticatedActorSeparately|TestCraftRunAdmissionRejectsCrossActorReplayAndCrossTenantActor|TestCraftRunAdmissionFailsClosedWithoutActorAndForLegacyReplay|TestGenericRunAdmissionDefaultsActorToOwnerForCompatibility)$' -count=1`
- GREEN result: repository package passed (`ok`, 22.409s).
- `gofmt -d` on the three owned files produced no changes; scoped `git diff --check` passed.

The repository-wide package rerun remains with the coordinating task as specified; this report does not claim it passed.
