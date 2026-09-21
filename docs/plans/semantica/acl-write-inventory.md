# A01 KB semantic ACL write inventory

This inventory covers the Go-authoritative KB scope. All listed invalidations
commit before the following business write. A business failure may leave an
extra epoch; that is conservative and requires scope reissue. Delivery also
recomputes current membership, effective share role, live documents and denials.
The application container decorates every listed writer with the durable I02
invalidator, including when semantic query serving is disabled.

| Service entry / callers | First repository mutation | Affected owner scope | Ordering evidence |
|---|---|---|---|
| `tenantMemberService.AddMember`; direct admin invite, `tenantInvitationService.Accept` / `AcceptByToken`, auth bootstrap | `TenantMemberRepository.Create` | Owned KBs plus source KBs shared to any current organization of the changed tenant | `TestSemanticScopeTenantMutation/add`; `TestSemanticScopeInvitationAcceptance`; repository `TestSemanticScopeEpochFanout` |
| `tenantMemberService.EnsureOwner`; registration and tenant creation/bootstrap | `TenantMemberRepository.Create` only when absent | Same tenant fanout; existing membership is a no-op | `TestSemanticScopeTenantMutation/ensure-owner` |
| `tenantMemberService.UpdateRole` | `UpdateRole` / `DemoteOwnerAtomically` | Same tenant fanout; same role is a no-op | `TestSemanticScopeTenantMutation/role`, `/demote-owner`, `/noop` |
| `tenantMemberService.RemoveMember`; admin removal, self-leave, rollback | `SoftDelete` / `RemoveOwnerAtomically` | Same tenant fanout | `TestSemanticScopeTenantMutation/remove`, `/remove-owner` |
| `tenantService.DeleteTenant`; tenant delete API and account-creation rollback | Repository transaction soft-deleting every tenant membership then tenant | Owned and shared source KBs, before membership deletion hides impact | `TestSemanticScopeTenantDelete`; deleted requester and owner tenant live checks in `TestSemanticScopeRejectsDeletedUserAndTenant` |
| `userService.DeleteUser`; account delete and registration rollback | `UserRepository.DeleteUser` | Union of owned/shared source KBs for all active membership tenants; one increment per source scope | `TestSemanticScopeUserDelete`; repository `TestSemanticScopeEpochUserUnion`; live user and inactive-user checks in `TestSemanticScopeRejectsDeletedUserAndTenant` |
| `kbShareService.ShareKnowledgeBase` | `KBShareRepository.Create`, duplicate fallback `Update` | Actual source KB owner tenant + KB | `TestSemanticScopeKBSharePreInvalidation/create`, `/duplicate` |
| `kbShareService.UpdateSharePermission` | `KBShareRepository.Update` | Persisted source owner tenant + KB; unchanged permission is a no-op | `TestSemanticScopeKBSharePreInvalidation/update`, `/noop` |
| `kbShareService.RemoveShare` | `KBShareRepository.Delete` (soft-delete) | Source owner tenant + KB | `TestSemanticScopeKBSharePreInvalidation/remove` |
| `knowledgeBaseService.DeleteKnowledgeBase`; API delete, evaluation cleanup, temporary web-search KB cleanup | `KnowledgeBaseRepository.DeleteKnowledgeBase`; then best-effort share cleanup `DeleteByKnowledgeBaseID` and async physical cleanup | Loaded KB's owner tenant + KB | `TestSemanticScopeKBDelete`; existing delete/data-source/task-cancel regressions |
| `organizationService.AddTenantMember` | `OrganizationRepository.AddTenantMember` | Every active source KB share to organization | `TestSemanticScopeOrganizationMutation/add` |
| `organizationService.JoinByInviteCode` / `JoinByOrganizationID` via `joinAsViewerWithChecks` | `OrganizationRepository.AddTenantMember` | Same organization fanout; already joined is a no-op | `TestSemanticScopeOrganizationMutation/join` |
| `organizationService.RemoveTenantMember`; self-leave and admin removal | `OrganizationRepository.RemoveTenantMember` | Same organization fanout | `TestSemanticScopeOrganizationMutation/leave`, `/remove` |
| `organizationService.UpdateTenantMemberRole` | `OrganizationRepository.UpdateTenantMemberRole` | Same organization fanout | `TestSemanticScopeOrganizationMutation/role` |
| `organizationService.ReviewJoinRequest`, approved join/upgrade only | `AddTenantMember` / `UpdateTenantMemberRole`, then review-status write | Same organization fanout; rejection and terminal re-review do not bump | `TestSemanticScopeOrganizationMutation/approve-join`, `/approve-upgrade`, `/reject`, `/terminal` |
| `organizationService.DeleteOrganization` | Best-effort `KBShareRepository.DeleteByOrganizationID`, agent-share cleanup, then organization soft-delete | Same organization fanout, before cleanup can hide impact rows | `TestSemanticScopeOrganizationMutation/delete`; surviving shares of deleted organizations denied by `TestSemanticScopeRejectsDeletedOrganizationWithSurvivingShare` |
| `knowledgeService.CloneKnowledgeBase` / `ProcessKBClone` → `executeKnowledgeClone` | Planned target document deletes and `cloneKnowledge` creation/checkpoints | Both source and destination owner tenant + KB, before nonempty plan runs | `TestSemanticScopeTransferFailsBeforeCheckpoint/clone`; existing clone retry/resume regressions |
| FAQ clone via `cloneFAQKnowledgeBase` | FAQ target document/chunk mutations after source preflight | Both source and destination owner scope | `TestSemanticScopeTransferFAQFailsBeforeCheckpoint` |
| `knowledgeService.ProcessKnowledgeMove` | `moveOneKnowledge` / `UpdateKnowledgeForTransfer`, then resumable document checkpoints | Both source and destination owner tenant + KB, after batch preflight and before first item | `TestSemanticScopeTransferFailsBeforeCheckpoint/move`; existing move partial retry/preflight regressions |

No-op and excluded operations:

- Invitation creation, decline, revocation, share-link creation and token lookup
  do not create membership. Successful acceptance reaches guarded `AddMember`.
- Organization creation generates a new UUID, then inserts its creator's tenant
  membership. There cannot be existing scopes/shared KBs for the new organization;
  rollback of that just-created organization has the same empty impact.
- Organization name, avatar, discovery/invite settings, pending join/upgrade
  requests, and invitation-code renewal do not grant current KB read access.
- Same-KB folder metadata moves do not change readable document ownership;
  `TestSemanticScopeTransferFolderMoveExcluded` verifies no invalidation.
- Agent shares and session shares are not KB authorization grants. Temporary
  session documents have a separate repository and are absent from I02's KB
  revision/outbox path. Temporary KBs are explicitly refused by scope issuance.
- I02 tombstone/restore writes already bump in their business transaction.
  I05 owns future production document-revision integration; A01 never invents
  semantic revisions from timestamps or indexes missing revision rows.

Account lifecycle audit was incorporated in plan correction `6ec6ba99` (A01
worktree cherry-pick `d9f09d160`). Both deletion service paths now pre-invalidate.
Every Issue/Resolve/Validate also reloads the active user, requesting tenant and
owner tenant, as child memberships/KBs/shares can outlive identity soft deletion.
`UpdateUser`/`UpdateTenant` account-active changes are likewise denied by these
live checks; ordinary profile/config updates do not affect KB grants. Tenant
creation's direct rollback has no related rows/scopes yet and is excluded.

Audit commands: `rg` over production (excluding tests) for `TenantMember{`,
`tenant_members`, `organization_tenant_members`, `KnowledgeBaseShare{`,
`AddMember`, `EnsureOwner`, `UpdateRole`, `RemoveMember`, `AddTenantMember`,
`RemoveTenantMember`, `UpdateTenantMemberRole`, `DeleteByOrganizationID`,
`DeleteByKnowledgeBaseID`, and transfer checkpoints. MCP OAuth uses membership
read/count queries only; it is not another writer.

SQLite test limitation: organization review's existing `NOW()` SQL is PostgreSQL
specific. A01's migrated SQLite fixture registers a connection-local NOW clock
function so the real status SQL still executes; production repository SQL is
unchanged. This does not claim native SQLite portability of that existing flow.
