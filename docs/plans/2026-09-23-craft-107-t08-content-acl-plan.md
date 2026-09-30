# T08 Craft Content ACL and List Plan

> **For Codex:** Execute with Superpowers SDD, RED → GREEN → REFACTOR, exact checkpoint, independent Review and backend validation. Do not commit.

**Goal:** Make current T08 Task roles govern dedicated Craft list, workspace, input/run write, version read and download bytes through the real service. This is central-B unit B1; generic parent Session direct metadata, preview capability revocation, frontend mount and joined HTTP remain separate gates.

**Sources:** Approved Spec #107/#126; `docs/plans/2026-09-23-craft-107-t08-central-b-design.md`; reviewed T08 central-A `CraftAccessService` and `craft.TaskAccessChecker`; T01 fix3 reviewed/integrated checkpoint is a prerequisite because this Task owns `craft_session.go`. Integration Worktree already carries T01 interim source and T08 central-A provider.

**Global Constraints:** backend_implementer owns `internal/application/service/craft_session.go` and focused tests, narrow Task-list query port/code in the same service package, and `internal/container/container.go` only for `newCraftSessionService` injection plus focused provider tests. No T01 input-claim logic edits beyond preserving it; no T14 preview, generic session handler, T05 knowledge, router source, frontend, migrations, commits or subagents. Other agents share integration Worktree; preserve their edits. Recheck T01 fix3 file hashes before starting to avoid overwriting its newer copy.

**Review Focus:** Task authority comes from current active tenant membership incarnation and explicit grant, never tenant Admin alone. Owner, Collaborator, Viewer can TaskRead; only Owner/Collaborator can TaskWrite; cross-tenant/nonmember/revoked cannot list or get content. Check current permission on every version/file read before opening a reader; denied stream has zero bytes and no metadata leak. Dedicated list includes shared Craft Tasks with stable pagination and excludes revoked/stale grants. Existing non-Craft Session behavior and T01 selected-input/admission semantics are preserved.

## Task 1 — current Task role for service reads/writes

**Depends on:** T01 fix3 reviewed and integrated; T08 central-A reviewed. **Consumes:** `craft.TaskAccessChecker`, scope from authenticated handler, persistent Craft Task owner row. **Produces:** TaskRead/TaskWrite gates on Craft service methods.

1. RED: Add service tests for owner, collaborator, viewer, revoked, stale membership, same-tenant nonmember, tenant Admin without grant and cross-tenant on View, ListVersions, GetVersion, OpenVersionFile, AssociateInput and StartRun. Assert denied download returns no reader/bytes and revoke takes effect on next call.
2. GREEN: Inject concrete checker into `CraftSessionService` through the production provider. Resolve Task owner/workspace storage scope only after checking the authenticated caller's current Task action; retain caller identity for ACL. Require the checker for shared access paths; fail closed when missing rather than infer admin privilege. Preserve existing owner admission/idempotency and resource scope checks.
3. REFACTOR: Keep one narrow access helper and clear action mapping. Run focused T01/T08 service tests and existing Craft session tests, `git diff --check`, record exact results/hashes.

## Task 2 — grant-aware dedicated Craft list

**Depends on:** Task 1 action model. **Consumes:** `CraftAccessService.ListAccessibleTaskIDs` or a narrow query port. **Produces:** bounded paginated `GET /craft/sessions` including owner/granted Craft Tasks only.

1. RED: Add pagination tests mixing owned and granted Tasks across tenants, revoked/stale grant, duplicate owner+grant, equal timestamps and cursor replay. Assert no Task existence leak to Admin/nonmember.
2. GREEN: Query accessible Task IDs under current tenant/user and join to registered Craft sessions, then paginate deterministically with existing cursor semantics. Keep kinds/gate projection unchanged. Do not widen generic `/sessions` listing.
3. Verify focused service/container tests and a real authenticated HTTP list→workspace→version→download sequence with Viewer read and revoked denial if the current handler route harness supports it. Leave B5 full joined test to its own checkpoint. Capture complete changed-file checkpoint, actual HEAD/status and remaining B2/B3/B5 gates.

**Failure handling:** If current `ListAccessibleTaskIDs` cannot paginate safely without unbounded ID materialization, add a narrow server-side paginated query port with SQL filtering rather than filtering after a page. Any needed new index/migration must be proposed to controller first; do not edit shared migrations in this Task.

**Controller ownership addendum (2026-09-23):** The backend implementer may update existing Craft-specific service/HTTP test expectations that assumed tenant Admin could bypass an explicit Task grant, after recording exact files and previous assertions in the B1 report. This does not grant ownership of production handler/router code or generic Session Admin audit/channel semantics. The changed tests must prove Admin without Task grant is denied while owner/grantee behavior remains intact.
