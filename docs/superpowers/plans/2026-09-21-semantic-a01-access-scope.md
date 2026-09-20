# Semantica A01 Access Scope and Revocation Barrier Implementation Plan

> **For agentic workers:** REQUIRED WORKFLOW: Use the dispatching-parallel-agents workflow to keep A01 isolated from independent V03. Follow RED → GREEN for every behavior change. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Issue short-lived, Go-authoritative semantic access scopes, and invalidate any already-issued scope before a KB ACL mutation can take effect.

**Architecture:** The Go scope service derives a canonical snapshot from current caller membership, KB ownership, existing tenant/organization/share authorization, current semantic document revisions, and deletion denials. `scope_ref` is a compact HS256 signed capability. Authenticated Semantica resolution verifies the service identity and capability, recomputes live access, and fails closed on any epoch or hash change. ACL writers pre-invalidate all impacted `(owner_tenant, KB)` epochs before writing ACL state; if the subsequent write fails, conservative extra invalidation is safe. Final delivery recomputes the live scope hash, closing the small interval between pre-invalidation and the permission write.

**Tech Stack:** Go/Gin/GORM, existing tenant/organization/KB-share authorization services, I02 semantic epochs/revisions/denials, HS256 HMAC capability, current C01 `SemanticAccessScope` DTO.

**Spec:** `docs/specs/2026-09-11-semantica-graphrag-reasoning-design.md` §§3, 5–6; `docs/adr/0002-semantica-independent-service.md`; `docs/plans/2026-09-20-semantica-rebaseline.md` (Q1/Q3/Q11); A01 in `docs/plans/2026-09-11-semantica-03-access-model.md`.

## Global Constraints

- Go remains authoritative for tenant, KB, document, and permission decisions; no caller-supplied owner tenant or allow-document list is trusted.
- Every semantic record/scope is owner-tenant + KB scoped. For shared KB access, the scope key uses the source KB's owner tenant, not the requesting tenant.
- Scope issue, resolve, and final delivery validation fail closed on permission/database errors, token tampering, audience/purpose mismatch, expiry, epoch change, hash change, or current deletion denial.
- The semantic service identity is authenticated separately from the HMAC signing key. `SEMANTIC_SERVICE_TOKEN` authenticates internal resolution requests; it is never the scope signing key.
- `SEMANTIC_SCOPE_SIGNING_KEY` is a distinct secret of at least 32 bytes, supplied through environment and required when Semantica is enabled; it is never logged or committed.
- Scope snapshots include only readable documents with an active I02 semantic revision. Tombstoned/denied documents are excluded. Missing semantic revision rows are not inferred from `Knowledge.UpdatedAt`; they remain out of scope until I05 writes revisions transactionally.
- For a non-deleted document, `allow_retained_previous=true` to preserve the Spec's ordinary-update stale-generation behavior; deletion denials still block every prior revision. Sensitive-replacement hide metadata does not exist yet and must remain unsupported until I03/I04 add an explicit Go-owned barrier. `budget_ref` is an opaque caller-provided reference bound into the capability; A01 creates no model budget authority.
- For shared KB access, an absent organization membership is a normal deny; storage/permission-service errors are not “no share” and must propagate as fail-closed errors.
- Tenant membership and organization role changes affect multiple owner scopes. Enumerate all owned or shared-to-tenant KBs (for tenant changes) or all KBs shared to the organization (for organization changes); bump scopes in sorted order.
- Invalidation happens before ACL state is written. A failed later ACL write may still invalidate scopes; this causes re-issue but no permission expansion. A scope minted during that interval is rejected after the write because `ValidateDelivery` recomputes the canonical live hash.
- Scope change coverage: tenant member add/role change/removal and invitation acceptance; organization join/role/review/remove/delete; KB share create/permission update/remove; KB delete; cross-KB move/clone checkpoints. Session sharing and temporary session attachments are excluded because current semantic revisions/outbox cover KB-owned `Knowledge`, not session-scoped temporary content. If those attachments later enter the semantic graph, they require a separate session-scope model.
- No proto/C01 changes, model/provider call, permission bypass, production backend promotion, or claim that Q04 already uses final validation.

## Review Focus

- Shared KB: use source/owner tenant in the semantic scope while the requester's live membership/org/share caps still gate access.
- Scope minted before/during revoke or demotion: old capability must fail after epoch or canonical hash changes, even when issued during the pre-bump/write interval.
- Forged/tampered scope_ref: reject invalid HMAC, algorithm/version, subject, requester tenant, owner scope, purpose, budget ref, audience, hash, epoch, or expiry.
- Large/empty scope: sorted unique IDs and exact revisions hash deterministically; no active semantic revision returns an empty readable set, not all current `Knowledge` rows.
- Mutation fanout: tenant changes invalidate owned plus shared-to-tenant owner scopes; organization changes invalidate every shared source KB; KB share/delete and cross-KB move invalidate the exact owner scopes.

---

### Task 1: Add scope impact discovery and multi-scope epoch invalidation

**Files:**
- Modify: `internal/application/repository/semantic_outbox.go`
- Create: `internal/application/repository/semantic_scope_epoch.go`
- Create: `internal/application/repository/semantic_scope_epoch_test.go`
- Create: `internal/types/interfaces/semantic_scope.go`
- Modify: `internal/container/container.go`

**Interfaces:**
- `SemanticScopeInvalidator.InvalidateTenant(ctx, tenantID) error`, `.InvalidateOrganization(ctx, organizationID) error`, `.InvalidateKB(ctx, ownerTenantID, kbID) error`, and `.InvalidateTransfer(ctx, sourceScope, destinationScope) error`.
- `SemanticControlRepository.BumpSemanticEpochs(ctx, scopes) error` de-duplicates and lexically sorts valid scopes, then bumps every epoch in one short transaction. Empty scope list is a no-op; malformed scopes fail before SQL.
- Tenant impact query returns every non-deleted KB owned by the tenant plus every non-deleted source KB shared to an organization the tenant belongs to. Organization impact query returns every active source KB share to that organization. Both queries return source-owner `(tenant_id,kb_id)` keys and deduplicate them.

- [ ] **Step 1: Write failing epoch discovery tests**

Use a temporary SQLite DB with formal migrations. Seed tenant 1's `owned` KB, tenant 2's membership in `org-1`, one active `kb_shares` row from tenant 1's `shared` KB to `org-1`, and one unrelated `org-2` share. Assert `InvalidateTenant(2)` affects exactly `(1,shared)`, `InvalidateOrganization(org-1)` affects exactly `(1,shared)`, and `InvalidateTenant(1)` includes `(1,owned)`. Send duplicate/unsorted scopes directly to `BumpSemanticEpochs` and assert one increment per distinct key.

- [ ] **Step 2: Confirm RED**

Run: `go test ./internal/application/repository -run '^TestSemanticScopeEpoch' -count=1`

Expected: fail because scope impact queries and multi-scope invalidation do not exist.

- [ ] **Step 3: Implement deterministic scope fanout and bump**

Use joins over `knowledge_bases`, `organization_tenant_members`, and `kb_shares`; filter soft-deleted KB/share rows. For tenant impact include owner KBs and shares through active tenant-org membership. For organization impact include shares where `organization_id` matches. Call existing `BumpSemanticEpoch(tx, scope)` in one transaction sorted by `(tenant_id,kb_id)`.

- [ ] **Step 4: Run GREEN**

Run: `go test ./internal/application/repository -run '^TestSemanticScopeEpoch' -count=1`

Expected: scope-key sets and epoch counts match, duplicate scopes increment once, and SQL/overflow errors propagate.

### Task 2: Implement canonical scope snapshot and signed capability

**Files:**
- Create: `internal/application/service/semantic_scope.go`
- Create: `internal/application/service/semantic_scope_test.go`
- Modify: `internal/application/repository/semantic_outbox.go`
- Modify: `internal/application/repository/semantic_scope_epoch.go`
- Modify: `internal/application/service/kbshare.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/semantic_test.go`
- Modify: `internal/runtime/startup.go`
- Modify: `internal/container/container.go`

**Interfaces:**
- `SemanticScopeSnapshot` contains owner scope, subject ID, requester tenant ID, sorted allowed document IDs, per-document maximum source revisions, `AllowRetainedPrevious`, denied document revisions, permission epoch, scope hash, expiry, purpose, audience, and opaque budget reference.
- `SemanticScopeService.Issue(ctx, subjectID, ownerScope, purpose, budgetRef) (types.SemanticAccessScope,error)`, `.Resolve(ctx, scopeRef) (SemanticScopeSnapshot,error)`, `.ValidateDelivery(ctx, scope) error`.
- `newSemanticScopeFixture(t)` uses a real temporary SQLite database with formal migrations, inserts tenant/org/KB/share/member rows, semantic revisions/denials, and a fixed test-only signing key. Its test-only helpers are `ContextFor(userID,requesterTenant)`, `AddOwnedKB(ownerTenant,kbID)`, `AddMember(userID,tenantID,role)`, `AddSharedKB(ownerTenant,memberTenant,kbID)`, `AddActiveSemanticDocument(ownerTenant,kbID,documentID,revision)`, `AddDeniedDocument(ownerTenant,kbID,documentID,revision)`, `FailOrganizationRead(err)`, `Issue(userID,requesterTenant,kbID,purpose,budgetRef)`, `BumpEpoch(scope)`, `ResolveScope(scopeRef)`, and `IssueWithExpiry(userID,requesterTenant,kbID,purpose,budgetRef,expiry)`.
- I02 repository adds `ReadSemanticScopeState(ctx, ownerScope) (epoch uint64, activeRevisions map[string]uint64, denied map[string]uint64, err error)`. It reads `semantic_document_revisions` and `semantic_denials`; no Go business model stores duplicate semantic revisions.
- Scope hash is SHA-256 of canonical JSON containing owner scope, subject, requester tenant, purpose, budget reference, epoch, sorted allowed IDs/revisions, sorted denials, and per-document `allow_retained_previous`; expiry and signature are bound separately in signed claims.
- `SEMANTIC_SCOPE_SIGNING_KEY` is added to `SemanticServiceConfig`, overridden only from that environment variable, marked sensitive in startup diagnostics, and validated only when `semantic.enabled=true`; require at least 32 non-whitespace bytes.

- [ ] **Step 1: Write failing scope tests**

```go
func TestSemanticScopeRejectsEpochChange(t *testing.T) {
    f := newSemanticScopeFixture(t)
    scope := f.Issue("member-a", 20, "shared-kb", types.SemanticAccessPurposeSearch, "budget-ref-1")
    f.BumpEpoch(types.SemanticScopeKey{TenantID: 10, KBID: "shared-kb"})
    require.ErrorIs(t, f.Service.ValidateDelivery(f.Context, scope), ErrSemanticScopeChanged)
}

func TestSemanticScopeRejectsTamperingAndExpiry(t *testing.T) {
    f := newSemanticScopeFixture(t)
    scope := f.Issue("member-a", 10, "kb-a", types.SemanticAccessPurposeSearch, "budget-ref-1")
    tampered := scope
    tampered.ScopeHash = "attacker-hash"
    require.ErrorIs(t, f.Service.ValidateDelivery(f.Context, tampered), ErrSemanticScopeInvalid)
    expired := f.IssueWithExpiry("member-a", 10, "kb-a", types.SemanticAccessPurposeSearch, "budget-ref-1", time.Unix(1, 0))
    require.ErrorIs(t, f.Service.Resolve(f.Context, expired.ScopeRef), ErrSemanticScopeExpired)
}

func TestSemanticScopeSharedKBUsesOwnerTenantAndExcludesDeniedRows(t *testing.T) {
    f := newSemanticScopeFixture(t)
    f.AddSharedKB(10, 20, "shared-kb")
    f.AddActiveSemanticDocument(10, "shared-kb", "doc-visible", 4)
    f.AddDeniedDocument(10, "shared-kb", "doc-deleted", 5)
    scope := f.Issue("member-a", 20, "shared-kb", types.SemanticAccessPurposeSearch, "budget-ref-1")
    snapshot, err := f.Service.Resolve(f.Context, scope.ScopeRef)
    require.NoError(t, err)
    require.Equal(t, uint64(10), snapshot.Scope.TenantID)
    require.Equal(t, []string{"doc-visible"}, snapshot.AllowedDocumentIDs)
    require.True(t, snapshot.AllowRetainedPrevious)
}

func TestSemanticScopeFailsClosedWhenOrganizationLookupFails(t *testing.T) {
    f := newSemanticScopeFixture(t)
    f.AddSharedKB(10, 20, "shared-kb")
    f.FailOrganizationRead(errors.New("organization store unavailable"))
    _, err := f.Service.Issue(f.ContextFor("member-a", 20), "member-a", types.SemanticScopeKey{TenantID: 10, KBID: "shared-kb"}, types.SemanticAccessPurposeSearch, "budget-ref-1")
    require.ErrorIs(t, err, ErrSemanticScopeUnavailable)
}
```

- [ ] **Step 2: Confirm RED**

Run: `go test ./internal/application/service ./internal/config -run 'TestSemantic(Scope|Enabled)' -count=1`

Expected: scope-service symbols/tests fail because the issuer, signer, and I02 scope-state read do not exist.

- [ ] **Step 3: Derive current access and build canonical snapshot**

Load the KB without assuming the request tenant owns it; compare the signed owner scope to `KnowledgeBase.TenantID`. Require the authenticated `Caller.UserID` to equal `subjectID`, load the caller's current tenant membership/role, and use `KBShareService.CheckTenantKBPermission` for shared KBs. In `CheckTenantKBPermission`, ignore only the `ErrOrgMemberNotFound` sentinel for an organization that does not grant access; propagate all other organization lookup errors. Only after access succeeds, list the source KB's `Knowledge` rows by the owner tenant and intersect IDs with I02 active semantic revisions; exclude current denials and set `AllowRetainedPrevious=true` for non-denied live documents. All repository/permission failures return errors and do not mint a scope.

- [ ] **Step 4: Implement HMAC capability, resolver, and delivery recheck**

Use a fixed claims struct and `golang-jwt/jwt/v5` HS256 with no algorithm fallback; include token version, subject, requester tenant, owner tenant/KB, purpose, audience, budget ref, epoch, scope hash, issued-at, and expiry. Set a five-minute maximum TTL. Issue sets C01 metadata from the same claims. Resolve verifies signature/version/audience/expiry, rehydrates current member role from signed subject/requester tenant, rebuilds current snapshot, and compares epoch/hash. ValidateDelivery parses the capability and exact-matches every field in `SemanticAccessScope` before rebuilding and comparing current live access. Live documents retain the prior published generation by default; current deletion denial excludes all prior generations. Never reveal signing key or ACL query errors.

Register `repository.NewSemanticControlRepository` and the invalidator provider in the container. Scope issue/resolve handlers are opt-in with `semantic.enabled`; epoch invalidation remains available to ACL writers whenever I02 migrations are installed.

- [ ] **Step 5: Run GREEN**

Run: `go test ./internal/application/service ./internal/config -run 'TestSemantic(Scope|Enabled)' -count=1`.

Expected: forged, expired, wrong owner, shared-owner, denied-revision, membership failure, and epoch/hash change tests pass; no proto/generation files change.

### Task 3: Add the service-authenticated internal resolver route

**Files:**
- Create: `internal/handler/semantic_internal.go`
- Create: `internal/handler/semantic_internal_test.go`
- Modify: `internal/router/routes_infra.go`
- Modify: `internal/router/router.go`
- Modify: `internal/container/container.go`

**Interfaces:**
- `POST /api/v1/internal/semantic/scopes/resolve` accepts only JSON `{"scope_ref":"..."}` and returns `SemanticScopeSnapshot` JSON.
- The route requires exact `Authorization: Bearer <SEMANTIC_SERVICE_TOKEN>` and `X-WeKnora-Audience == SemanticServiceConfig.Audience`, checked with constant-time token comparison. User JWT and API keys are not fallback credentials.
- Handler calls `SemanticScopeService.Resolve`; invalid/expired/changed scope returns a non-leaking unauthorized response; internal errors fail closed and return service unavailable.

- [ ] **Step 1: Write failing route tests**

Test missing token, wrong token, wrong audience, malformed/expired capability, and valid configured service request. Assert unauthenticated calls do not invoke the service and responses never contain the signing key or service token.

- [ ] **Step 2: Confirm RED**

Run: `go test ./internal/handler ./internal/router -run '^TestSemanticInternalScope' -count=1`

Expected: fail because the resolver route is not registered.

- [ ] **Step 3: Implement internal route and handler**

Register only under the internal route prefix; authenticate service token and audience before JSON decoding/resolution; propagate request cancellation; never attach ordinary user RBAC as a substitute.

- [ ] **Step 4: Run GREEN**

Run: `go test ./internal/handler ./internal/router -run '^TestSemanticInternalScope' -count=1`.

Expected: only the configured authenticated service resolves a valid scope.

### Task 4: Add fail-closed invalidation to direct KB share and delete mutations

**Files:**
- Modify: `internal/application/service/kbshare.go`
- Modify: `internal/application/service/knowledgebase.go`
- Modify: `internal/application/service/semantic_scope.go`
- Modify: `internal/container/container.go`
- Test: relevant KB-share/KB-delete tests and `semantic_scope_test.go`

- [ ] **Step 1: Write failing tests**

Assert share create, duplicate-share permission update, permission change, share removal, and KB soft-delete advance the source owner KB epoch before the mutation repository is called. A deliberately failing mutation must leave share/KB data unchanged but keep the epoch increment; a failing epoch repository must prevent the mutation entirely.

- [ ] **Step 2: Confirm RED**

Run: `go test ./internal/application/service -run '^TestSemanticScope(KBShare|KBDelete)' -count=1`.

- [ ] **Step 3: Pre-invalidate exact owner scope before existing writes**

Resolve the source KB owner, call `InvalidateKB(ctx, ownerTenant, kbID)`, then execute the existing authorized mutation. Preserve duplicate, audit, soft-delete, and async cleanup behavior. Do not bump for reads/no-op writes.

- [ ] **Step 4: Run GREEN**

Run the same focused service test command; expected: invalidation is first, failures remain fail-closed, and the prior error/result contract is unchanged.

### Task 5: Add fail-closed invalidation to tenant/org and cross-KB transfer mutations

**Files:**
- Modify: `internal/application/service/tenant_member.go`
- Modify: `internal/application/service/organization.go`
- Modify: `internal/application/service/knowledge_clone_move.go`
- Modify: relevant service constructors/container wiring
- Create: `docs/plans/semantica/acl-write-inventory.md`
- Test: tenant member, organization, transfer tests, and `semantic_scope_test.go`

- [ ] **Step 1: Write failing fanout and ordering tests**

For tenant member add/role-change/removal, assert `InvalidateTenant` includes all owned KBs and source KB scopes shared to organizations the tenant belongs to. For org join/leave/role/review/delete, assert `InvalidateOrganization` includes all shared source KB scopes. For move/clone, assert `InvalidateTransfer` receives both source and destination owner scopes before the first per-document transfer checkpoint. A same-KB folder move must not invalidate. Invitation issue/revoke alone must not invalidate; successful invitation acceptance uses `AddMember` and does.

- [ ] **Step 2: Confirm RED**

Run: `go test ./internal/application/service -run '^TestSemanticScope(Tenant|Organization|Transfer|Invitation)' -count=1`.

- [ ] **Step 3: Wire every listed ACL mutation before its first state change**

Validate the caller and target first, then invalidate, then perform the existing write. Include `ReviewJoinRequest` only when it approves a role/join change; it should not invalidate when rejecting or re-reviewing a terminal request. Org deletion invalidates shared KB scopes before any best-effort share cleanup. Cross-KB transfer invalidates source and destination before the resumable move/clone starts; `ValidateDelivery` rechecks live content after later checkpoints. Inventory every callsite in `acl-write-inventory.md` with service method, repository mutation, affected owner scopes, ordering test, and explicit exclusion for session shares/temporary attachments.

- [ ] **Step 4: Run GREEN**

Run: `go test ./internal/application/service -run '^TestSemanticScope(Tenant|Organization|Transfer|Invitation)' -count=1` and `go test ./internal/application/repository -run '^TestSemanticScopeEpoch' -count=1`.

Expected: grants/revocations invalidate all impacted scopes before mutation; failed mutation may conservatively invalidate but cannot leave an old capability valid after a committed change; excluded session-only operations remain unchanged.

### Task 6: Final A01 verification and evidence

- [ ] **Step 1: Run focused and broad regression suites**

Run: `go test ./internal/application/service ./internal/application/repository ./internal/handler ./internal/router ./internal/config -run 'TestSemanticScope|TestSemanticInternalScope|TestSemantic.*Scope' -count=1`.

Also run full existing member, organization, KB-share, KB delete, knowledge-transfer, and config test packages without `-run` filters.

- [ ] **Step 2: Audit ACL write inventory**

Use `rg` on production callers of all listed mutations; reconcile every caller with `acl-write-inventory.md`. Confirm the scope guard is injected in the application container. If any permission writer mutates scopes without pre-invalidation or current-hash revalidation, A01 is not complete.

- [ ] **Step 3: Commit and record evidence**

Update `docs/plans/semantica/progress.md` with exact commands, exits, environment, review result, and limitations. Commit scoped A01 files as `feat(semantic): a01 trusted access scopes and revocation barrier`.

## Rulings

- `scope_ref` uses a Go-only HMAC key distinct from the service bearer token; the internal endpoint additionally checks service token and audience. This keeps claims tamper-evident while avoiding persistence of large allowlists; if the signer secret is misconfigured, scope issue/resolution fails closed.
- ACL epochs are pre-bumped before ACL writes because existing membership/share repositories own private transactions. If the ACL write later fails, an extra epoch increment only forces harmless re-issue; final live-scope hash comparison catches a scope minted during that interval. Cost if wrong: users may need one re-issued scope after a failed permission update.
- Session sharing and temporary attachments do not enter the current KB-owned semantic revision/outbox. They remain outside A01 and must not be silently converted to owner-KB permissions. Cost if wrong: if a future semantic indexing path includes temporary/session documents, a distinct session scope model is required before that feature ships.

## Completion Boundary

A01 provides Go issue/resolve/final-validate and conservative invalidation for KB-relevant ACL writes. It does not implement A02 Python authorized-subgraph filtering, A03 model budget enforcement, Q04 delivery integration, temporary session attachment authorization, or a claim that user-facing GraphRAG is enabled.
