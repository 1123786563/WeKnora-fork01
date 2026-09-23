# T08-1: Craft grant after tenant removal and rejoin

## Evidence and requirement reading

- Approved Spec #107 says a Craft Task is private by default, the owner explicitly adds Collaborators/Viewers, and all Craft operations enforce those roles (`docs/specs/2026-09-23-craft-web-artifact-spec.md`, stories 28–31, implementation decision at line 67). Ticket #126 requires owner add/revoke, same-tenant membership, and immediate revocation across list/direct/preview/download. Neither text explicitly names tenant removal followed by rejoin. Treat non-revival as an **inference from private-by-default and explicit owner grant**, not a newly quoted acceptance clause.
- `CONTEXT.md` defines Task Grant within member permissions (line 339). ADR-0004 fixes Task identity to Session identity; ADR-0009 keeps tenant authorization authoritative. The T08 brief produces explicit grants and revocation audit, and reserves central migrations to T20.
- `CraftAccessService.Role` checks only whether *some current active* `tenant_members` row exists for `(tenant_id,user_id)`, then looks up `craft_task_grants` by `(tenant_id,session_id,user_id)`. `ListAccessibleTaskIDs` repeats the active-member check and joins grants on those three keys. `ListMembers` also shows grants without filtering current membership. The grant row has no membership-generation identifier.
- Tenant removal soft-deletes the active row; `AddMember` and `EnsureOwner` create a new row. The partial unique index in PostgreSQL permits a replacement `(user,tenant)` row after deletion (`migrations/versioned/000043_tenant_rbac.up.sql`). `RemoveMember` does not touch Craft grants. Therefore access disappears while absent, then the old grant becomes live when the same user rejoins. The existing T08 test covers explicit `Revoke`, not this lifecycle.

## Recommendation

Bind each Craft grant to the **specific tenant membership row ID** present when the owner grants access. `Role` and task-list filtering must require that exact row to remain active and undeleted. A rejoin creates a new ID and cannot reactivate the old grant. The owner must explicitly grant access again. `ListMembers` should show only effective grants, or clearly distinguish stale records, so owners do not mistake stale grants for current authority. Keep historical grant/revoke audit entries.

This keeps the invariant in the Craft access boundary and does not couple generic tenant removal to Craft. A removal hook that deletes grants is smaller syntactically but has failure/race windows under the current post-delete best-effort hook pattern, and would add a new dependency from tenant membership service into Craft. Timestamp comparison (`grant.created_at >= member.joined_at`) is also weaker than row identity because of clock precision and concurrent operations.

## Proposed files and migration

- T08-owned: `internal/application/service/craft_access.go`, `internal/application/service/craft_access_test.go`; optionally `internal/handler/session/craft_access_test.go` for the full API journey. The service can keep its public `TaskAccessChecker` interface unchanged.
- T20-owned migration numbering: add `membership_id` to `craft_task_grants` in both PostgreSQL versioned and SQLite migration tracks, with an index only if query evidence warrants one. Coordinate an exclusive migration lock with the controller. Existing deployed grant rows have unknown membership generation; **fail closed** for null IDs and require owner regrant. Do not backfill from current membership: a member may already have left and rejoined before migration. For a fresh installation, amend the new Craft frontier migration only if it has not shipped; otherwise add forward migrations. Document the one-time access reset.
- `Grant` must read the active target membership row and persist its ID with the grant in the same DB transaction. Authorization reads compare stored ID to the current active row. Recheck/lock as supported, but the identity comparison is the essential race fence: a concurrent removal and rejoin cannot make a grant bound to the former ID valid. The owner membership check remains mandatory.

## Focused regression

At the service/API seam: owner grants Viewer; Viewer can read/list/preview; soft-delete Viewer tenant membership; all reads deny; create a new active membership for the same user and tenant; direct read, list, preview and download still deny; owner explicitly regrants; access returns; verify audit and effective member list. Use the real schema or migration-backed SQLite test, not a handcrafted grant table that omits `membership_id`.

## Decision point

This is a security-boundary interpretation of #126, not an explicit lifecycle requirement. The proposed identity binding is the smallest robust design if the controller accepts that removed members lose old Task invitations permanently. If product intends invitations to survive tenant departure, record that policy in the Spec first; the current behavior should not remain accidental.
