# T08 migration fixture correction report

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- Baseline HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`
- Owned source file: `internal/application/service/craft_access_test.go`
- Pre-edit SHA-256: `1e7954a0e6197b2198e51174387fc9e2cb3774530cb20f46a11d94d4aa2c4c27`
- Post-edit SHA-256: `27d65a60c9fcc5d3df0d8c08720c51080bbf8be080f63865a347134f182906fe`

## Change

Removed the duplicate `tenant_members` insertion in `TestCraftAccessMigrationJourney`. `openCraftSessionDB` already seeds active `u1` and `u2` memberships. The journey's later membership revocation, rejoin, incarnation, grant, and audit assertions remain unchanged. No production source or migration changed.

## Evidence

- Before edit: `go test ./internal/application/service -run '^TestCraftAccessMigrationJourney$' -count=1` failed at fixture setup with `UNIQUE constraint failed: tenant_members.user_id, tenant_members.tenant_id`.
- After edit: the same isolated command passed.
- After edit: `go test ./internal/application/service -run '^TestCraft(T08Journey|TaskLookupIsTenantAndSessionScopedIndependentOfGrant|TaskLookupFailsClosedForMissingOrDeletedSession|AccessMigrationJourney)$' -count=1` passed.
- `git diff --check -- internal/application/service/craft_access_test.go` passed.
- Task-scoped patch: `2026-09-24-craft-107-t08-migration-fixture-fix-task-local.patch`.

## Scope note

The worktree and source file contained unrelated concurrent modifications before this task. This task changed only the redundant insert line in its owned source file. The attached task-local patch records only that line removal.
