# T08 denial audit fix1 — independent scoped re-review

**Scope.** Read the approved Craft web Artifact Spec, `CONTEXT.md`, ADR-0004, the T08 denial-audit plan, the original Task 1 review, fix1 plan/report, checkpoint manifest, pre/post snapshots, task-local patch, and current joined router test. This is a read-only review of the one-file fix1 delta; no OCR was run.

## Findings

No remaining critical, high, or medium finding in the fix1 scope. Original finding F1 is resolved.

The test-only preview route injects a controlled tenant and Viewer principal rather than running production credential middleware (`internal/router/craft_b5_joined_test.go`, `testPreviewRouter.Use`). It proves the real persistent `TaskPreview` ACL and audit behavior after the preview gates are opened for this test. It does not prove production preview authentication or production preview availability; the report and comments correctly keep those claims separate. This is a scope limit, not a defect in the requested fix.

## Spec compliance verdict: PASS for the scoped fix

- `assertDenied` takes an audit snapshot around **each** reachable direct Session, version-list, version-detail, and file-download request. Pre-grant Viewer, Admin without a grant, revoked Viewer/Collaborator, and Viewer with stale membership each require exactly one new durable `craft.access_denied` row with tenant 1, the current actor, Task/session `task-b5`, denied outcome, and the exact allowlisted `read` detail. Tenant-mismatched and same-tenant outer-guard refusals require zero Task-targeted rows. Response denial, absence of private identifiers/content, and zero file-service opens remain asserted (`internal/router/craft_b5_joined_test.go`, `assertDenied`).
- The production-configured preview issue route retains closed gates and now requires zero Task denial rows for each attempted request. The separate test-gated route uses the real `CraftAccessService`; `CraftPreviewService.Issue` reaches `RequireTaskAccess(..., TaskPreview)` before version lookup or storage. A revoked Viewer request requires 403, exactly one correctly attributed row with the fixed `preview` detail, no file-service open, and no file bytes (`internal/router/craft_b5_joined_test.go`, test-gated preview block; `internal/application/service/craft_preview.go:238-258`). The production preview configuration and source code are untouched.
- The bounded row assertions use the exact JSON allowlist, so query, body, filename, and content cannot satisfy them. The original joined test's other access-mutation and grant/revoke audit assertions remain intact.

## Code quality verdict: PASS for the scoped fix

The before/after assertions address F1's aggregate-count blind spot without changing the production authorization path. The checkpoint's pre/post SHA-256 values match the saved files, its post hash `4798ce71963621f0d5f512155b0680e044572158fcb504bf66331888c46c2951` matches the current test, and patch hash `04f5141e7c741fe4536d10d2dbd6367be4c0bcd2fdf2179cb559098a4beeb786` matches the manifest. The patch changes only `internal/router/craft_b5_joined_test.go`.

Independent current-tree checks passed: `go test ./internal/router -run '^TestCraftB5JoinedCurrentProduction$' -count=1`; `go test ./internal/router -run 'CraftB5JoinedCurrentProduction|CraftAccessProductionSessionAssemblyAuthAndAudit' -count=1`; `go test ./internal/handler/session -run '^TestCraftPreview' -count=1`; `go test ./internal/application/service -run '^TestCraftAccessDenialAuditIsTenantScopedAndBounded$|^TestCraftT08Journey$' -count=1`; and `git diff --check -- internal/router/craft_b5_joined_test.go`. The broader service ACL selector's previously reported workspace-fixture failure was outside this fix and was not rerun for this scoped verdict.
