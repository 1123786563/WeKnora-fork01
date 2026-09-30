# T08 ACL Workspace fixture fix2 — independent review

## Verdict

- **Spec compliance: PASS** for the assigned test-fixture correction. The helper uses the persisted Workspace identity scoped by tenant, Session, and owner; the three invalid primary-key rewrites are gone. Existing ACL, version, and download assertions remain.
- **Code quality: PASS** for this checkpoint. No actionable correctness, security, concurrency, data-consistency, or architecture finding in the task-local delta.

## Evidence

- Reviewed the fix1 stop report, fix2 plan, implementation report, checkpoint, task-local patch, and current test files against `docs/specs/2026-09-23-craft-web-artifact-spec.md`, `docs/adr/0004-task-is-session.md`, and `CONTEXT.md`. The approved model uses Session as Task identity, one persistent Workspace per Craft Task, immutable versions, and Owner/Collaborator/Viewer authorization.
- `publishCraftVersion` in `internal/application/service/craft_session_test.go` now selects `id` from `craft_workspaces` with `tenant_id = scope.TenantID`, `session_id = scope.SessionID`, and `owner_id = scope.UserID`. It checks the query error and requires exactly one row before deriving the Version ID and publishing. All three call sites pass the owner scope.
- The patch removes one unchecked `UPDATE craft_workspaces SET id` from the version/download test and two from the ACL tests. No production or migration file appears in the task-local patch. Inspection of the surrounding tests found the prior permission, version, file-content, download, and cross-session assertions intact.
- Current SHA256 hashes equal the recorded post-task hashes for both files. Reversing the four task-local hunks in memory produced both recorded pre-task hashes (`f5c74739…` and `a2ad6212…`). This confirms the patch accounts exactly for this task's two-file delta and excludes the earlier shared-worktree edits in `craft_session_test.go`.
- Independently ran `go test ./internal/application/service -run 'TestCraftSessionTaskAccessGatesContentWritesAndDownloadBytes|TestCraftSessionTaskAccessRejectsStaleMembershipAndAdminWithoutGrant|TestCraftSessionVersionsAndDownloadChain' -count=1`: PASS. Ran `go test ./internal/application/service -run 'Craft.*Access|SessionShare' -count=1`: PASS. `git diff --check` on the two files: PASS.

## Findings

None. This verdict covers the fix2 task-local delta and the named selectors, not the full integration branch or all Craft behavior.
