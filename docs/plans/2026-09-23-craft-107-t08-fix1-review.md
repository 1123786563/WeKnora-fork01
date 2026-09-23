# T08-1 independent service re-review

Reviewed 2026-09-23. Read-only scope: the two-file uncommitted T08-1 checkpoint in `/Users/wuyongjun/.codex/worktrees/craft-107-t08/WeKnora-fork01` at HEAD `dad8236768b1de88764c8780e017cc3615db1aa2`. Manifest SHA-256 `8944d1ecb0e979b46445c4b432789164db375d469764b4fa8755b3c3fe50e899`; tracked diff SHA-256 `06da4aac62c6cf87381720248a49694a44cf01e47d92796fed36888efaff53ab`. Both live source files match the manifest SHA-256 entries (`ab524813...` service, `40b5e0f3...` test). `git diff --check` passed. I did not run OCR or repeat the implementer's behavioral test commands.

Authority: approved Craft Spec stories 28–31 and all-operation role rule; `CONTEXT.md` private Task, Owner, Collaborator, Viewer and Task Grant definitions; ADR-0004 and ADR-0009; Issue #126/T08 brief; original T08 review and T08-1 rejoin analysis; fix plan/report; independently reviewed migration checkpoint-02. Non-revival after tenant rejoin is an inference from private-by-default and explicit Owner invitation, not a separately quoted Spec clause.

## Verdict

- **T08-1 Spec compliance: PASS at the service seam.** `Grant` records the current target `tenant_members.id` inside its grant/audit transaction and updates it on regrant. `Role` requires that same active membership ID; `ListAccessibleTaskIDs` joins grant to the live membership ID; `ListMembers` omits stale grants. The migration-backed test exercises removal, a new membership ID, denial before regrant, and restoration after regrant. The reviewed PG/SQLite Craft migrations supply the non-null ID column.
- **T08-1 code quality: PASS with the test portability limitation below.** Missing or multiple active membership rows fail closed in direct checks and list preflight; database errors propagate rather than producing a role. A removal/rejoin race after Grant reads the old ID can leave a newly committed grant ineffective, but cannot reactivate the old invitation. The service does not claim to lock a membership for the duration of downstream byte serving.
- **Issue #126 completion: NOT YET VERIFIED.** The prior T08-2 high integration gate and T08-3 medium HTTP/audit test gate remain separate. This service checkpoint does not establish that central Task list/direct/preview/download routes, including ticket redemption and byte serving, enforce a fresh check. The controller must validate the integrated HTTP journey before the full ticket can pass.

## Finding

### T08F1-1 — Low, test portability — SQLite rejoin test changes the legacy membership schema

**Evidence / affected file:** `internal/application/service/craft_access_test.go:118-129` drops `idx_tenant_members_user_tenant_unique` before inserting the replacement member. The real SQLite base migration `migrations/sqlite/000000_init.up.sql:357-358` creates an unconditional unique index on `(user_id, tenant_id)`, while PostgreSQL `migrations/versioned/000043_tenant_rbac.up.sql:40-43` uses a partial index limited to undeleted rows. `tenantMemberService.AddMember` creates a new row after a deleted membership; on unmodified SQLite schema, that insertion will hit the unique index. The independently reviewed Craft migration only adds `craft_task_grants.membership_id` and does not address the legacy index.

**Impact:** The migration-backed test proves the Craft access policy against the intended replacement-row state, but it does not prove that a removed user can actually rejoin through the normal SQLite membership service. Its comment discloses the index change, so this is a portability/coverage limitation rather than a hidden passing test. PostgreSQL's partial index supports the intended journey.

**Smallest defensible correction:** Track SQLite membership index parity in the owning tenant-membership migration work: replace the unconditional index with a partial `WHERE deleted_at IS NULL` index, then run a migration-backed remove → `AddMember` → Craft access journey without dropping the index in the test. Do not fold that unrelated base-schema change into the T08-1 service checkpoint.

## Evidence and limits

- The new service code uses `membership_id` consistently in `craftTaskGrant`, Grant upsert, direct role lookup, task listing, and member projection (`craft_access.go:31-40, 76-110, 141-150, 175-219`). The composite grant key stays `(tenant_id, session_id, user_id)`, so an explicit regrant replaces the stale row and binds it to the replacement ID.
- `TestCraftT08Journey` and `TestCraftAccessMigrationJourney` assert that a removed/rejoined Viewer cannot read/preview or list the Task, that stale grants do not appear in the member list, and that a fresh Owner Grant restores access. The migration-backed journey also compares the stored grant ID with the replacement membership ID (`craft_access_test.go:68-98, 103-155`). The implementer's report records RED before the fix and focused GREEN after the reviewed migration copy; those test outputs were not independently rerun here.
- `ListAccessibleTaskIDs` first rejects a missing/ambiguous active membership, then performs an ID-matched join in its listing query. `Role` propagates query errors and returns `ErrForbidden` for no matching grant. Neither lookup falls back to ordinary tenant-admin authority.
- The original T08-2/T08-3 findings remain central integration obligations, not regressions attributed to this two-file fix. T10 must separately authorize original source access with the requesting member's current source permission.
