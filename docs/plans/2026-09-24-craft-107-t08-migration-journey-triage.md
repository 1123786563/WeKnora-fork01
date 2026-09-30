# T08 membership migration journey failure triage

Date: 2026-09-24. Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`. Read-only triage; only this report was added. No source/test edits or subagents.

## Verdict

`TestCraftAccessMigrationJourney` deterministically fails at fixture setup on the current integration state. This is a stale fixture conflicting with the shared `openCraftSessionDB` seed and SQLite's legacy membership unique index. It is not a B2 behavior failure and not a shared-memory/concurrent-test artifact. The test fails before exercising its membership deletion/rejoin or grant migration assertions.

## Revision and hashes

HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`.

```text
1e7954a0e6197b2198e51174387fc9e2cb3774530cb20f46a11d94d4aa2c4c27  internal/application/service/craft_access_test.go
b9690a1bd0bd5a44a7c13aeae5e3ea268fbd1a161230c5c40740ba8198e410c5  internal/application/service/craft_session_test.go
57597ca2cc42742dc39ed923ac032e0c128f5ecfd990120a0c8c95e618e8e2c9  migrations/sqlite/000000_init.up.sql
b58a351d5b69eaad4f192b9ddaf533fdd5ac79825e71c30fff6c80263359e6f8  migrations/sqlite/000110_craft_web_artifact_frontier.up.sql
5eb2223f1396c005802193cfb3be4a9b570935a69d2521b42f5870bb3e6dd374  migrations/versioned/000043_tenant_rbac.up.sql
9a5b429f34e9e746573defc63ff5fd1302850c83e08fbe0ae416ab17532ba04c  migrations/versioned/000189_craft_web_artifact_frontier.up.sql
```

## Exact run

Command:

```text
go test ./internal/application/service -run '^TestCraftAccessMigrationJourney$' -count=1 -v
```

Result: **FAIL**, exit 1, package duration 2.077s. Error:

```text
craft_access_test.go:162: Received unexpected error:
UNIQUE constraint failed: tenant_members.user_id, tenant_members.tenant_id
TestCraftAccessMigrationJourney (0.81s)
```

Failing statement at `internal/application/service/craft_access_test.go:162`:

```sql
INSERT INTO tenant_members (...)
VALUES (1,'u1',...), (1,'u2',...)
```

`openCraftSessionDB`, called at line 159, applies SQLite migrations and then seeds active tenant membership rows for both `(tenant_id=1,user_id='u1')` and `(1,'u2')` in `craft_session_test.go:61-63`. SQLite base migration `000000_init.up.sql:357-358` defines unconditional unique index `idx_tenant_members_user_tenant_unique` on `(user_id, tenant_id)`, so the explicit inserts at line 162 collide before the test constructs `CraftAccessService` or reads/writes `craft_task_grants`.

The subsequent fixture itself already accounts for the known SQLite/PostgreSQL index difference: lines 173-177 drop the unconditional index before soft-deleting `u2` and inserting a replacement membership. That handling occurs too late to fix the earlier duplicate seed insert.

The T08 membership migration is not the source of this failure. SQLite `000110_craft_web_artifact_frontier.up.sql` adds `craft_task_grants.membership_id INTEGER NOT NULL`; PostgreSQL `000189_craft_web_artifact_frontier.up.sql` adds `BIGINT NOT NULL`. Neither modifies `tenant_members` nor introduces the unique index. The PostgreSQL tenant RBAC migration `000043_tenant_rbac.up.sql` uses a partial active-row unique index, but this failing test is explicitly running against migrated SQLite via `openCraftSessionDB`.

Historical `2026-09-23-craft-107-t08-fix1-backend-validation.md` records this test passing before the helper seeded these same memberships. The current helper plus current journey test form a deterministic fixture mismatch. The package test uses a temporary on-disk database path from `t.TempDir()`, so this is not leftover shared test state.

## Scope conclusion

This failure is unrelated to B2 direct Session behavior. It does mean the current migration membership-incarnation regression is not green and should not be cited as current passing evidence until its fixture is reconciled by the owning implementation lane. I made no changes. This triage does not assess B2, B3, or broader Craft behavior.
