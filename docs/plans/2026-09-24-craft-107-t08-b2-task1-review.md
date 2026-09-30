# T08 B2 Task 1 independent Spec and quality review

Date: 2026-09-24. Scope: task-local `GetSession` Craft read branch and `session_craft_read_test.go` only. This is not a B3, B5, or full T08 verdict. Read-only source review; no OCR rerun.

## Verdict

**Scoped Spec compliance: PASS. Scoped code quality: PASS with a behavioral-test gap.** No confirmed blocking correctness or security finding in the task-local patch. The branch classifies an active tenant-scoped Craft registration before the generic owner/Admin path, evaluates `TaskRead` with the caller scope, reads only the tenant-scoped Session row, and rechecks registration before returning. Errors and denied access return `ErrSessionNotFound` with a nil Session. The non-Craft path remains `loadSessionForRead`; no generic write, message, router, or list path was changed.

## Finding

**Low — production HTTP/auth integration is untested in this slice.** Evidence: `session_craft_read_test.go` uses a map-backed `TaskRunAccess` fake and calls `sessionService.GetSession` directly; there is no handler or real `CraftAccessService` test in the task-local patch. Impact: the tests prove the service branch and non-leaking service return, but do not prove the production route's authenticated principal, actual membership/grant checks, or 404 response body. Smallest correction: add the planned B5 authenticated real-service route journey covering granted Viewer, ungranted Admin, revoked/stale member and cross-tenant denial, including absence of Session metadata in denied HTTP responses. This is a stated B5 gate, not a reason to expand this B2 source patch.

## Evidence and limits

- Reviewed approved `docs/specs/2026-09-23-craft-web-artifact-spec.md` (private default, explicit roles, all Craft operations), `docs/adr/0004-task-is-session.md`, `CONTEXT.md` Task roles, central-B seam map, T08 gap map, B2 plan/report and exact task-local patch.
- `session.go`, new test file, and task-local patch SHA-256 match the task report: `e3c08887c24ba03d6088d1ab1b2c7adacdb138a1c8ce44b88b9862cce3ba8241`, `8962f59ced45ca503d591b86e465f72e4efc98f075e6aa502927b1ac06302b83`, and `e86724caf25fac777f1808dad57dbb0dbebd080c361b9fde5159ab44a792ccc2`. `git apply --reverse --check` passes, confirming the task-local patch is present. The concurrent `TaskRunAccess` constructor injection is baseline, outside this patch.
- `go test ./internal/application/service -run '^TestGetSession(CraftTask|NonCraft|AllowsAdminToOpenAPIKeySessions)' -count=1` passed. `git diff --check` passed. Existing `GetSessionAllowsAdminToReadAPIExternalUserSession` and other selected tests completed within the broader run.
- Broader `SessionShare|Craft.*Access` selection failed only at `TestCraftAccessMigrationJourney` (`craft_access_test.go:162`) with `UNIQUE constraint failed: tenant_members.user_id, tenant_members.tenant_id`. That fixture/migration failure is outside the B2 owned files and prevents calling the broad selector green.
- Classification distinguishes an active Craft registration from an ordinary Session. An independently removed registration row on a still-active Session is indistinguishable from an ordinary Session at this seam; no such deletion path was found in the reviewed application code. The planned joined route test should exercise supported deletion/revocation behavior, while any future registration deletion design must preserve a durable Craft identity or tombstone to keep fail-closed semantics.
