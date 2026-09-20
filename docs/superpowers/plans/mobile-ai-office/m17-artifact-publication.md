# M17 Artifact Publication Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development`. Track every step with checkboxes.

**Goal:** Publish a ready artifact version to a current-space resource as an immutable copy with an ACL independent of Task sharing.

**Architecture:** M10 supplies readable immutable artifact versions, and M15 supplies Task share state only as a source ACL. This slice persists a publication snapshot and publication ACL, exposes authenticated publish/read commands, and renders those results in `mobile-next`; router, DI, exports, root layout, migration number, root manifest, and lockfile are controller-owned serial assembly.

**Tech Stack:** Go/Gin/GORM, existing `repository.ArtifactVersionStore`, Expo 55/React Native 0.83, Jest.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; Parent #3.

## Global Constraints

- Blocked by M10 FilesRead and M15 TaskShare; consume their integrated contracts, not their branch-local types.
- Scope every server request with backend, accountId, tenantId, and generation; a Task is `sessionId` and an artifact Run belongs to that Session.
- Publication reuses the immutable source blob but creates a new logical publication object reference and independent ACL; it does not physically copy bytes. Therefore M12 reserves no additional blob bytes for logical-reference publication, while the retention row prevents source cleanup until every publication reference is removed. A future physical-copy target must reserve source size before copy; failed staging/ACL commit deletes only its unreferenced logical ref. Unsharing a Task never revokes a publication grant.
- Target is only a current-tenant artifact resource; no public URL, cross-tenant target, credits/billing, second Web, or Paseo daemon.
- Keep shared router/container/exports/lock/root-layout/migration ownership with the controller; this track writes only the files below and submits an assembly request.

## Review Focus

1. A stale generation or foreign tenant cannot publish or read a digest.
2. Replaying the same idempotency key returns the same immutable publication without a second byte reservation.
3. Removing TaskShare leaves publication-reader download authorization intact.
4. A modified source artifact needs a new publication; it cannot mutate the prior digest.

### Task M17: immutable current-space artifact publication

**Files:**
- Create: `internal/types/artifact_publication.go`, `internal/application/repository/artifact_publication.go`, `internal/application/service/artifact_publication.go`.
- Create: `internal/application/service/artifact_publication_test.go`, `internal/handler/session/artifact_publication.go`.
- Create: `apps/mobile-next/src/features/artifacts/publication/publication.ts`, `apps/mobile-next/tests/features/artifact-publication.test.tsx`.
- Controller only: router registration, container wiring, package exports, root layout, migration number, root manifests, lockfile.

**Interfaces:**
- Consumes: `ArtifactVersionStore.ReadableArtifactVersion(ctx, tenantID, sessionID, id) (ArtifactVersion, error)`; M15 `TaskShareAuthorizer.CanRead(ctx, tenantID uint64, sessionID, accountID string) error`.
- Produces (proposed ports): `ImmutableBlobRef{ObjectKey string; Digest string; Size int64}`; `PublicationBlobStore.StageLogicalReference(ctx, tenantID uint64, source ImmutableBlobRef, publicationID string) (PublicationObjectRef,error)` where `PublicationObjectRef{PublicationObjectKey,SourceObjectKey,Digest string; Size int64}`; `FinalizeReference(ctx, publicationID string) error`; `RollbackReference(ctx, publicationID string) error`; `PublicationGrantStore.Grant(ctx, publicationID, subjectID, action string) error`; `CanRead(ctx, publicationID, subjectID string) error`.
- Produces (proposed): `PublishArtifact(ctx context.Context, scope PublicationScope, request PublishArtifactRequest) (PublicationView, error)` and `ReadPublication(ctx context.Context, scope PublicationScope, publicationID string) (PublicationView, error)`.
- Proposed request: `PublishArtifactRequest{RequestID, SessionID, ArtifactVersionID, TargetResourceID string; ExpectedVersion int64}`. Proposed view includes `PublicationID`, `SourceDigest`, `TargetResourceID`, `ACLVersion`, and `State`.

- [ ] **Step 1: Write the RED service and mobile-contract tests.**

```go
func TestPublishArtifactCreatesIndependentReferenceAfterTaskUnshare(t *testing.T) {
    published := publish(t, ownerScope, sourceVersion, "req-17", "space-resource")
    grantPublication(t, published.PublicationID, readerID, "read")
    revokeTaskShare(t, sessionID, readerID)
    got := readPublication(t, readerScope, published.PublicationID)
    require.Equal(t, sourceVersion.Digest, got.SourceDigest); require.NotEqual(t, sourceVersion.ObjectKey, got.PublicationObjectKey)
}
```

```ts
it("shows the returned immutable digest and does not infer access from task sharing", async () => {
  render(<PublicationCard publication={publication} />);
  expect(screen.getByText(publication.source_digest)).toBeTruthy();
});
```

- [ ] **Step 2: Run RED and record the missing behavior.**

Run: `go test ./internal/application/service -run 'TestPublishArtifact' -count=1`.

Run: `cd apps/mobile-next && npm test -- --runInBand artifact-publication.test.tsx`.

Expected: compilation/behavior fails because the publication command and renderer do not yet exist; no test may be skipped.

- [ ] **Step 3: Implement the smallest server contract.**

```go
func (s *PublicationService) PublishArtifact(ctx context.Context, scope PublicationScope, in PublishArtifactRequest) (PublicationView, error) {
    source, err := s.versions.ReadableArtifactVersion(ctx, scope.TenantID, in.SessionID, in.ArtifactVersionID)
    if err != nil { return PublicationView{}, err }
    return s.store.CreateWithReferenceAndOwnerGrant(ctx, scope, in, source)
}
```

`CreateWithReferenceAndOwnerGrant` validates current tenant resource ownership, stages an independent logical object reference (no byte reservation/copy), writes publication/ACL/reference-retention rows atomically, finalizes the reference after commit, and rolls back the staged reference on transaction failure. It returns the original publication for identical `RequestID`; the publication download handler authorizes `PublicationGrantStore.CanRead`, never TaskShare. Future physical-copy publication requires a separate approved slice and quota contract.

- [ ] **Step 4: Add HTTP/mobile mapping without shared assembly edits.**

Implement a handler that derives `PublicationScope` from authenticated account/tenant and rejects client-supplied scope fields. A separate read handler checks publication grant before resolving `PublicationObjectKey`. `publication.ts` calls the proposed endpoint with `request_id` and `expected_version`, displays `conflict` and `forbidden` distinctly, and never fabricates a successful copy.

- [ ] **Step 5: Run GREEN, negative cases, and controller handoff.**

Run: `go test ./internal/application/service -run 'TestPublishArtifact' -count=1`.

Run: `cd apps/mobile-next && npm run typecheck && npm test -- --runInBand artifact-publication.test.tsx`.

Verify foreign tenant, viewer write, digest mutation, independent grant after Task unshare, staged-reference rollback, and idempotent replay. Byte-quota exhaustion is out of scope for this logical-reference path; a future physical-copy ticket must test M12 reservation/copy rollback. Submit exact handler/route/container/migration requirements to the controller; do not edit those shared files.

## Review Gate

Reject an implementation that lacks independent object reference/grant/read path, cascades TaskShare revocation into publication ACL, leaks staged refs after failure, or reports mock/Jest output as cloud/device evidence.
