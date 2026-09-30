# T08 denial audit Task 1 report

## Result

Added durable `craft.access_denied` records at `CraftAccessService.CheckTaskAccess`. A refusal is recorded only after a tenant-scoped active Craft Task lookup succeeds. Missing, deleted, tenant-mismatched, malformed, and database-error lookups do not become Task-targeted denial rows. `Role` remains read-only, avoiding a duplicate write from role-display callers. Each independent `CheckTaskAccess` decision writes at most one row; there is no time-window suppression.

The audit row contains tenant, authenticated actor, `scope_type=session`, Task/Session scope ID, `outcome=denied`, and JSON details with only an allowlisted Task action and fixed `policy_denied` reason. No request path/query, body, target, file bytes, or arbitrary error text is persisted. Audit insertion failure is logged with the fixed action name and does not change the original authorization denial. No repository helper or migration was required.

## Changed files

- `internal/application/service/craft_access.go` — write the bounded denial row after a proven policy refusal.
- `internal/application/service/craft_access_test.go` — cover known Task denials, hidden/malformed lookups, repeated independent decisions, allowed reads, inactive membership, and audit-sink failure. Existing journey assertions continue to verify grant/revoke success rows exactly once.
- `internal/router/craft_b5_joined_test.go` — verify durable rows through the production joined router; denied Viewer access mutation adds exactly one row and query/body/target markers do not enter details. Existing file denial checks continue to assert no storage opens or bytes.
- `docs/plans/2026-09-24-craft-107-t08-denial-audit-task1-report.md` — this report.
- `docs/plans/2026-09-24-craft-107-t08-denial-audit-task1-checkpoint.json` and checkpoint `pre/`, `post/` snapshots — exact pre/post state evidence.
- `docs/plans/2026-09-24-craft-107-t08-denial-audit-task1-task-local.patch` — owned-source delta from captured pre-state.

## TDD and verification

- RED: `go test ./internal/application/service -run '^TestCraftAccessDenialAuditIsTenantScopedAndBounded$' -count=1` failed because the expected two `craft.access_denied` rows were absent.
- GREEN: `go test ./internal/application/service -run '^TestCraftAccessDenialAuditIsTenantScopedAndBounded$|^TestCraftT08Journey$' -count=1` — PASS.
- `go test ./internal/router -run 'CraftB5JoinedCurrentProduction|CraftAccessProductionSessionAssemblyAuthAndAudit' -count=1` — PASS.
- `git diff --check -- internal/application/service/craft_access.go internal/application/service/craft_access_test.go internal/router/craft_b5_joined_test.go` — PASS.
- Broader planned selector `go test ./internal/application/service -run 'Craft.*Access|SessionShare' -count=1` — two failures in existing `craft_session_acl_test.go` workspace setup (`craft not found: workspace ws-of-...`), unrelated to denial-audit assertions. The same output also showed the new policy audit tests passing.
- Initial compile attempt was blocked while concurrent `craft_draft_head.go` imports were incomplete; that shared file stabilized without changes from this task and focused verification then passed.

No commit was made. The shared Worktree contains unrelated concurrent changes; they are excluded from this task's local patch. Independent review is pending the parent orchestrator.
