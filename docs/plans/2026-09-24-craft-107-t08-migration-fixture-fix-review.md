# Independent review — T08 migration fixture correction

**Verdict:** PASS for the task delta. No actionable spec or quality finding.

## Scope and evidence

- Reviewed the task plan, implementation report, checkpoint, task-local patch, migration-journey triage, approved Craft spec, `CONTEXT.md`, and the affected test/helper. The patch removes only the duplicate `tenant_members` insert from `TestCraftAccessMigrationJourney` in `internal/application/service/craft_access_test.go`.
- `openCraftSessionDB` already inserts active `u1` and `u2` membership rows. The removed insert repeated those same `(user_id, tenant_id)` keys under SQLite's unique index, preventing the journey from reaching its assertions.
- The post-edit source SHA-256 is `27d65a60c9fcc5d3df0d8c08720c51080bbf8be080f63865a347134f182906fe`, matching the checkpoint. `git diff --check -- internal/application/service/craft_access_test.go` passed.
- Independently ran `go test ./internal/application/service -run '^TestCraft(T08Journey|TaskLookupIsTenantAndSessionScopedIndependentOfGrant|TaskLookupFailsClosedForMissingOrDeletedSession|AccessMigrationJourney)$' -count=1`: passed (`ok`, 1.419s). The implementation report records the isolated pre-edit UNIQUE failure and post-edit pass.

## Spec compliance

**PASS.** The approved Craft spec requires explicit Task Owner grants and private-by-default Task access; `CONTEXT.md` distinguishes Task Owner, Viewer, and Task Grant. The corrected journey retains the initial denial, explicit grant, revocation upon tenant membership deletion, denial after rejoin with a new membership ID, omission of the stale grant from effective membership, and access only after a fresh Owner grant. It still checks the persisted grant's `membership_id` equals the replacement row ID. The SQLite/PostgreSQL index-parity comment and index drop remain at the rejoin step; the correction does not relax production uniqueness or alter migrations.

## Code quality

**PASS for this narrow fixture change.** The patch removes the setup collision without changing production code or weakening the membership-incarnation assertions. The focused migrated journey and adjacent access tests pass. This verdict covers the task-local one-line delta, not the unrelated concurrent worktree changes or broader Craft integration.
