# T08 ACL Workspace fixture fix2 Task 1 report

Status: implemented and verified; no commit.

## Change

- `publishCraftVersion` now queries `craft_workspaces` by tenant ID, session ID, and owner ID, checks the query error, and requires exactly one persisted row before using its ID for the Version ID and WorkspaceID.
- Removed the three unchecked Workspace primary-key rewrites from the ACL and version/download tests.
- Preserved all existing ACL, version, and download assertions.

## RED → GREEN evidence

RED command:

`go test ./internal/application/service -run 'TestCraftSessionTaskAccessGatesContentWritesAndDownloadBytes|TestCraftSessionTaskAccessRejectsStaleMembershipAndAdminWithoutGrant|TestCraftSessionVersionsAndDownloadChain' -count=1`

Result: failed all three tests at `publishCraftVersion` with `craft not found: workspace ws-of-<session-id>`.

GREEN focused command: same selector after the change; passed (`ok`, 4.768s).

GREEN broad command:

`go test ./internal/application/service -run 'Craft.*Access|SessionShare' -count=1`

Passed (`ok`, 3.222s).

`git diff --check -- internal/application/service/craft_session_test.go internal/application/service/craft_session_acl_test.go` passed.

## Checkpoint

- Pre-task SHA256 `craft_session_test.go`: `f5c747391eff316411f2226e4b9b92fcdea3428f988eeac58b817b002b2ffbee` (file already contained unrelated shared-worktree edits; these were preserved).
- Post-task SHA256 `craft_session_test.go`: `d37f3de2ea035874b5a86da0ada4769da65fd75e49ecb1a82af8e9f86e37572a`.
- Pre-task SHA256 `craft_session_acl_test.go`: `a2ad621214e884cc4dc0009dd30b472fb23ae1e5404bc23439c07ce4840e4031` (pre-existing untracked test file).
- Post-task SHA256 `craft_session_acl_test.go`: `3c075b2db2b6482683f5d2ed2444f2e9a0a666f6cf39dd4188b7e6e0903f4115`.
- Task-local patch: `docs/plans/2026-09-24-craft-107-t08-acl-workspace-fixture-fix2-task-local.patch`.

`craft_session_test.go` contained unrelated edits before this task. They were left intact and excluded from the task-local patch. No production or migration file changed.
