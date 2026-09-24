# T08 denial audit fix1 report

## Result

Addressed review finding F1 in the joined HTTP test only. Each direct Session/version/download request that reaches the Task ACL now checks the per-request audit row delta and asserts its tenant, authenticated actor, `craft.access_denied` action, session/Task scope, denied outcome, and exact allowlisted read details. These assertions run for the pre-grant Viewer, Admin without a grant, revoked Viewer/Collaborator, and Viewer with stale membership. The same helper asserts no Task row for tenant-mismatched and outer-membership-gated requests. Existing response privacy and zero file-opener checks remain.

The first stronger run exposed a response-contract distinction: known-Task ACL denials can be rendered as `404`, while some `403` responses (same-tenant nonmember) are produced by the outer route role guard before the Task ACL. The test now declares which actors reach the ACL; reached requests require exactly one row regardless of the masked 403/404 status, and earlier gates require zero Task-targeted rows.

Production preview configuration remains unchanged and disabled. Its issue route explicitly asserts that the disabled gate writes no Task denial row. A separate, clearly test-only authenticated preview issue route uses controlled enabled browser/no-egress gates with the real persistent access checker; a revoked Viewer request reaches `TaskPreview`, returns 403, creates exactly one bounded preview denial row, and opens/returns no file bytes. This is not evidence that production preview is enabled or isolated.

## Changed files

- `internal/router/craft_b5_joined_test.go` — per-request direct ACL audit assertions and a separate test-gated preview ACL request.
- `docs/plans/2026-09-24-craft-107-t08-denial-audit-fix1-report.md` — this report.
- `docs/plans/2026-09-24-craft-107-t08-denial-audit-fix1-checkpoint.json` and `...-checkpoint/{pre,post}/` — exact source snapshots and hashes.
- `docs/plans/2026-09-24-craft-107-t08-denial-audit-fix1-task-local.patch` — fix-only source delta.

No production source was changed and no commit was made.

## Verification

- `go test ./internal/router -run '^TestCraftB5JoinedCurrentProduction$' -count=1` — PASS after refining the 403/hidden-404/outer-guard expectations.
- `go test ./internal/router -run 'CraftB5JoinedCurrentProduction|CraftAccessProductionSessionAssemblyAuthAndAudit' -count=1` — PASS.
- `go test ./internal/handler/session -run '^TestCraftPreview' -count=1` — PASS.
- `go test ./internal/application/service -run '^TestCraftAccessDenialAuditIsTenantScopedAndBounded$|^TestCraftT08Journey$' -count=1` — PASS.
- `git diff --check -- internal/router/craft_b5_joined_test.go` — PASS.

Initial RED run failed while validating the new per-request assertions because it treated all response `404`s as pre-ACL and all `403`s as Task ACL decisions. The observed row deltas showed hidden ACL-denial responses and pre-ACL membership denials; after encoding those distinctions, all focused tests pass.

Independent re-review is pending parent orchestration. No broad service selector was rerun; the Task1 report/review records its unrelated workspace fixture failure.
