# M18 Knowledge Publication Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development`. Track every step with checkboxes.

**Goal:** Import an immutable M17 publication into one explicitly selected, authorized knowledge base in the current tenant.

**Architecture:** M17 is the sole source record; M04 provides the knowledge-base selection/read contract. The service authorizes the selected KB, imports a digest-bound resource, and reports indexing state to mobile; controller serially owns shared API assembly and migrations.

**Tech Stack:** Go/Gin/GORM, existing knowledge service/access checks, Expo 55/React Native 0.83, Jest.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; Parent #3.

## Global Constraints

- Blocked by M04 AgentsKnowledge and M17 ArtifactPublication; do not recreate their lists or publication bytes.
- Scope is backend/accountId/tenantId/generation and Task is sessionId. Reject a KB selected from another tenant even if its ID is valid.
- A target is a named knowledge base only. No public/export link and no automatic fan-out to every KB.
- Import records publication digest/version and target KB; later source changes require a new M17 publication and a new import.
- Controller owns root router/container/exports/lock/root layout/migration numbering. This ticket keeps feature ownership under `apps/mobile-next/src/features/knowledge-publication`.

## Review Focus

1. A reader of the Task cannot import into a KB without KB write permission.
2. Tenant mismatch returns forbidden/not-found without revealing target metadata.
3. Idempotent replay returns the existing import rather than duplicate indexed content.
4. A failed index/import leaves a visible failure state and does not falsely claim publication succeeded.
5. Source digest remains the M17 snapshot when the original Task artifact later changes.

### Task M18: publication-to-KB import

**Files:**
- Create: `internal/application/service/artifact_publication_knowledge.go`, `internal/application/service/artifact_publication_knowledge_test.go`.
- Create: `internal/handler/session/artifact_publication_knowledge.go`.
- Create: `apps/mobile-next/src/features/knowledge-publication/importPublication.ts`, `apps/mobile-next/tests/features/knowledge-publication.test.tsx`.
- Controller only: route/DI/export/root/migration/manifest/lock assembly.

**Interfaces:**
- Consumes: M17 proposed `ReadPublication(ctx, scope, publicationID)`; M04 `KnowledgeBaseAccess.CanWrite(ctx, tenantID uint64, accountID, knowledgeBaseID string) error`.
- Produces (proposed): `ImportPublication(ctx context.Context, scope PublicationScope, in ImportPublicationRequest) (PublicationImportView, error)`.
- Proposed request: `ImportPublicationRequest{RequestID, PublicationID, KnowledgeBaseID string; ExpectedVersion int64}`; view contains `ImportID`, `PublicationID`, `KnowledgeBaseID`, `Digest`, `IndexState`, `Revision`.

- [ ] **Step 1: Write RED behavior tests.**

```go
func TestImportPublicationUsesSelectedCurrentTenantKnowledgeBase(t *testing.T) {
    got := importPublication(t, ownerScope, publicationID, currentKB, "m18-idem")
    require.Equal(t, publishedDigest, got.Digest)
    require.ErrorIs(t, importPublicationErr(t, ownerScope, publicationID, foreignKB, "m18-foreign"), ErrForbidden)
}
```

```ts
it("submits only the selected KB and renders server indexing state", async () => {
  await chooseKnowledgeBase("kb-1"); await pressImport();
  expect(api.importPublication).toHaveBeenCalledWith(expect.objectContaining({ knowledge_base_id: "kb-1" }));
});
```

- [ ] **Step 2: Run RED.**

Run: `go test ./internal/application/service -run 'TestImportPublication' -count=1`.

Run: `cd apps/mobile-next && npm test -- --runInBand knowledge-publication.test.tsx`.

Expected: both fail before the import contract exists.

- [ ] **Step 3: Implement authorization and import persistence.**

```go
func (s *PublicationKnowledgeService) ImportPublication(ctx context.Context, scope PublicationScope, in ImportPublicationRequest) (PublicationImportView, error) {
    p, err := s.publications.Read(ctx, scope, in.PublicationID); if err != nil { return PublicationImportView{}, err }
    if err := s.kbAccess.CanWrite(ctx, scope.TenantID, scope.AccountID, in.KnowledgeBaseID); err != nil { return PublicationImportView{}, err }
    return s.imports.CreateOnce(ctx, scope, in, p.Digest)
}
```

Map accepted import work to an explicit pending/ready/failed `IndexState`; preserve the server error and source digest.

- [ ] **Step 4: Add handler and mobile feature.**

The handler derives scope from auth and accepts only `publication_id`, `knowledge_base_id`, `request_id`, and `expected_version`. The feature consumes M04’s actual KB options, requires selection, disables duplicate submit while pending, and shows current server state after refresh.

- [ ] **Step 5: GREEN and handoff.**

Run: `go test ./internal/application/service -run 'TestImportPublication' -count=1`.

Run: `cd apps/mobile-next && npm run typecheck && npm test -- --runInBand knowledge-publication.test.tsx`.

Verify owner/no-KB-write/foreign-tenant/idempotency/index-failure cases; send exact assembly request to controller.

## Review Gate

Reject global KB lists, implicit target selection, copied mutable source paths, or any claim that an indexed test double proves real cloud indexing.
