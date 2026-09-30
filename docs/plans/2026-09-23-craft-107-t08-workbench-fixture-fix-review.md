# T08 active-membership Workbench fixture fix: independent scoped review

Date: 2026-09-23. Read-only review of `t08-workbench-fixture-fix-{plan,report}.md`, the three-file test diff, the reviewed T08 active-membership admission predicate, and the five-failure full repository log from the T19 budget-pause checkpoint. Integration HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. No production/test edits, staging, delegation or OCR.

## Exact files and verdict

| Test file | Full-content SHA-256 |
| --- | --- |
| `internal/application/repository/agent_run_driver_test.go` | `3c2edb14a356734bd0c25acb171da4bfe4174f02e8999b8bee961e9a6831c24f` |
| `internal/application/repository/artifact_version_test.go` | `a1eaf1545df6069f5236ba9eed9f4166a61b7acac8b7275e074f743e34814cc0` |
| `internal/application/repository/workbench_http_integration_test.go` | `c3da63d5c19a782bcfe097382792a0ece6fc33d4878202b4e01e2f581f8a6865` |

The live hashes match the implementation report. The diff contains only three fixture inserts: tenant 2/u2 in the driver and artifact tests, and tenant 1/u1 in the shared Workbench HTTP fixture. No production code, test assertion, denial expectation or unrelated fixture changed.

- **Scoped Spec compliance: PASS.** The legitimate test actors now satisfy the reviewed active-tenant-membership admission rule without granting any foreign tenant/user access or weakening production authorization.
- **Scoped code quality: PASS.** Each inserted row uses the migrated `tenant_members` table with matching tenant/user, `admin` role and `active` status, before the corresponding admission. The four prior admission-conflict tests and focused actor admission tests pass. No scoped finding was identified.
- **Full T08/integration: NOT VERIFIED.** This is fixture-only. The full repository package has not been rerun at this checkpoint, and the separate lifecycle cancellation event-count failure remains open.

## Evidence and test limits

The reviewed production predicate checks `tenant_members` on exact `tenant_id` and actor `user_id`, `status='active'`, non-deleted membership, and active/non-deleted user (`agent_run.go:247-252`). The three added fixture rows satisfy only their own legitimate actor/tenant pair (`agent_run_driver_test.go:117-118`, `artifact_version_test.go:251-253`, `workbench_http_integration_test.go:47`). The existing cross-tenant assertions remain in place, so the fixture change does not change the expected isolation result.

I independently ran the four formerly conflicting tests plus six focused actor admission tests in one repository command; all passed. `git diff --check` on the three reviewed files passed. The prior full-package log recorded five failures: these four admission conflicts and `TestCancelRunReleasesSlotAndDeleteFences`'s event-count assertion. The latter does not use these fixture changes and is not claimed fixed. A fresh full-package pass is still required for integration completion.
