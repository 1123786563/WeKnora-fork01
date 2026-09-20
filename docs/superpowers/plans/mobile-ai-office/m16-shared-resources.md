# M16 Shared Resources Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task. Track checkboxes as work proceeds.

**Goal:** Expose Task files and artifacts to a read-only shared member only while both the Task grant and resource authorization remain active, and revoke access promptly.

**Architecture:** Compose M10’s safe reader with M15’s Task ACL and existing artifact authorization rather than duplicating storage or runtime state. Resource access is a server-side intersection: current tenant, active Task reader grant, resource belongs to session/workspace, and ordinary resource visibility. Mobile lists capability-filtered resources and drops cached entries on 403/404 revocation.

**Tech Stack:** Go/Gin/Craft artifact services, Expo/TypeScript/Jest.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; design `docs/superpowers/specs/2026-09-20-mobile-ai-office-design.md`.

## Global Constraints

- M10 and M15 are required; this slice adds read paths only, never writer/PTY/approval authority.
- Original workspace resources are not public and do not become independent published copies.
- Resource grants use the existing session/task authority; no secondary shared runtime, duplicated artifact store, or cross-space target is introduced.
- Controller owns any shared route/export/container integration; this track owns listed unique feature/service/handler files.

## Review Focus

- A reader grant alone cannot fetch a file/artifact attached to another session or workspace.
- Task revocation after a list response prevents the next download, even with a guessed stable file/artifact ID.
- A resource-specific authorization loss returns invisible/forbidden without revealing object metadata.
- Owner behavior remains unchanged for list/download while reader behavior never gains mutation links.
- Existing fixed-version publication ACL is not widened by a Task reader grant and remains accessible only under its own policy.

---

### Task 1: Revocable shared file and artifact reader vertical slice

**Files:**
- Create: `internal/application/service/mobileworkspace/shared_resources.go`
- Create: `internal/application/service/mobileworkspace/shared_resources_test.go`
- Create: `internal/handler/session/mobile_shared_resources.go`
- Create: `internal/handler/session/mobile_shared_resources_test.go`
- Create: `apps/mobile-next/src/features/shared-resources/SharedResourcesClient.ts`
- Create: `apps/mobile-next/tests/features/shared-resources/SharedResourcesClient.test.ts`
- Create: `apps/mobile-next/src/features/shared-resources/SharedResourcesState.ts`
- Create: `apps/mobile-next/tests/features/shared-resources/SharedResourcesState.test.ts`

**Interfaces:**
- Consumes: M10 `Reader.List/Read`, M15 `TaskSharing.Capabilities`, existing Craft version/artifact read authorization and `craft.Scope`.
- Produces: **proposed** `SharedResource{Kind "file"|"artifact", ID string, Name string, Bytes int64, DownloadURL string}`; `(*SharedResources).List(ctx context.Context, scope craft.Scope) ([]SharedResource,error)` and `Download(ctx,scope,id string) (io.ReadCloser,error)`; **proposed** `GET /sessions/:session_id/mobile-workspace/shared-resources` and `/shared-resources/:id/download`; **proposed** `SharedResourcesClient.list(sessionId,scope)`.

- [ ] **Step 1: Write authorization RED tests.**

```go
func TestSharedReaderCanOnlyReadResourcesBoundToGrantedTask(t *testing.T) {
  svc := sharedResourcesHarness(t, readerGrant("s1", "member"))
  list, err := svc.List(ctx, memberScope("s1")); require.NoError(t, err)
  require.Equal(t, []string{"file:report.txt", "artifact:v1/report.pdf"}, ids(list))
  _, err = svc.Download(ctx, memberScope("s1"), "file:other-session-secret")
  require.ErrorIs(t, err, craft.ErrNotFound)
}
func TestRevokedReaderCannotDownloadPreviouslyListedArtifact(t *testing.T) {
  h := handlerWithGrantThenRevoke(t, "s1", "member")
  require.Equal(t, 403, downloadShared(t,h,"member","s1","artifact:v1/report.pdf").Code)
}
```

- [ ] **Step 2: Run RED.**

Run: `go test ./internal/application/service/mobileworkspace ./internal/handler/session -run 'Test(SharedReaderCanOnlyReadResourcesBoundToGrantedTask|RevokedReaderCannotDownloadPreviouslyListedArtifact)' -count=1`
Expected: FAIL because shared-resource composition is absent.

- [ ] **Step 3: Write mobile RED tests.**

```ts
it("evicts listed shared resources after a revoke response", async () => {
  const state = new SharedResourcesState(clientWithDownload403());
  state.seed([{ id:"artifact:v1/report.pdf", kind:"artifact", name:"report.pdf", bytes:4 }]);
  await expect(state.download("artifact:v1/report.pdf")).rejects.toThrow("access revoked");
  expect(state.items).toEqual([]);
});
```

- [ ] **Step 4: Implement intersection authorization.**

```go
func (s *SharedResources) Download(ctx context.Context, scope craft.Scope, id string) (io.ReadCloser,error) {
  caps, err := s.shares.Capabilities(ctx,scope); if err != nil || !caps.CanRead { return nil, craft.ErrNotFound }
  resource, err := s.resources.FindBoundToSession(ctx,scope.TenantID,scope.SessionID,id); if err != nil { return nil, err }
  return s.resources.OpenAuthorized(ctx,scope,resource)
}
```

```ts
async download(id: string) { try { return await this.client.download(this.sessionId,id,this.scope); } catch (e) { if (isRevoked(e)) this.items=[]; throw e; } }
```

- [ ] **Step 5: Run GREEN gates.**

Run: `go test ./internal/application/service/mobileworkspace ./internal/handler/session -run 'Test(SharedReader|RevokedReader|MobileSharedResources)' -count=1`
Run: `cd apps/mobile-next && npm test -- --runInBand tests/features/shared-resources && npm run typecheck && npm run check:isolation`
Expected: Task/resource intersection, post-list revocation, and mobile eviction pass.

- [ ] **Step 6: Review and demo gate.** Grant member access to Task A with a file and an artifact; verify list/download. Revoke then verify new download fails and cached items clear. Attempt Task B’s resource ID and verify no metadata leaks. Keep fixed-version publish ACL tests separate.

## Execution Handoff

Implement in one fresh context. Do not add write endpoints or alter existing fixed-version publish ACLs. The controller performs final shared router/container/index integration after dependent tracks are green.
