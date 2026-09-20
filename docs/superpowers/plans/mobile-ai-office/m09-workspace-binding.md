# M09 Workspace Binding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task. Track checkboxes as work proceeds.

**Goal:** Bind an owner’s Task (the existing session ID) to exactly one durable Craft workspace and project it through a narrow mobile client.

**Architecture:** Reuse `craft.Workspace`, `CraftWorkspaceService`, `SessionSandboxBinding`, and the existing Workbench snapshot API. The server remains the authority for tenant, owner, session and generation; `CloudWorkspaceClient` only maps authenticated HTTP views into a mobile feature projection. This slice deliberately excludes file CRUD, quota, PTY, and provider suspend/resume.

**Tech Stack:** Go/Gin/GORM, existing Craft services, Expo/TypeScript/Jest.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; design `docs/superpowers/specs/2026-09-20-mobile-ai-office-design.md`.

## Global Constraints

- Task is `sessionId`; do not introduce a Task state machine or a second runtime.
- Scope is always backend-derived `(backend, accountId, tenantId, generation)`; server authorization never trusts a workspace ID.
- Reuse the existing persistent workspace and session binding; do not add file operations or a migration in this track.
- The fixed controller serially integrates router/container/index exports/root layout and migration numbering after this track.

## Review Focus

- A session belonging to another tenant must be invisible even if the workspace ID is guessed.
- A different member of the same tenant must not receive the owner binding from this write-capable endpoint.
- A stale mobile scope/generation must discard its response instead of replacing the current task view.
- Reopening the same owner session must return its original workspace ID rather than provisioning another one.
- A response that omits `session_id`, `workspace_id`, or positive revision must fail contract decoding.

---

### Task 1: Owner-scoped workspace binding vertical slice

**Files:**
- Create: `internal/application/service/mobileworkspace/binding.go`
- Create: `internal/application/service/mobileworkspace/binding_test.go`
- Create: `internal/handler/session/mobile_workspace_binding.go`
- Create: `internal/handler/session/mobile_workspace_binding_test.go`
- Create: `apps/mobile-next/src/features/workspace-binding/CloudWorkspaceClient.ts`
- Create: `apps/mobile-next/tests/features/workspace-binding/CloudWorkspaceClient.test.ts`
- Create: `apps/mobile-next/src/features/workspace-binding/useWorkspaceBinding.ts`
- Create: `apps/mobile-next/tests/features/workspace-binding/useWorkspaceBinding.test.ts`

**Interfaces:**
- Consumes: existing `craft.Scope{TenantID uint64, UserID string, SessionID string}`, `(*service.CraftWorkspaceService).Resolve(context.Context, craft.Scope) (craft.Workspace, error)`, and authenticated `craftScope(*gin.Context) (craft.Scope, bool)`.
- Produces: **proposed** `mobileworkspace.Binding{SessionID string, WorkspaceID string, Revision int64, Generation string}` and `(*BindingService).GetOrBind(ctx context.Context, scope craft.Scope) (Binding, error)`; **proposed** `GET /api/v1/sessions/:session_id/mobile-workspace` returning `{session_id,workspace_id,revision,generation}`; **proposed** `CloudWorkspaceClient.getBinding(input: { sessionId: string; scope: CloudWorkspaceScope; signal?: AbortSignal }): Promise<WorkspaceBinding>`.

- [ ] **Step 1: Write the server RED tests.**

```go
func TestBindingServiceReusesOwnerWorkspace(t *testing.T) {
  svc := bindingServiceWithCraftWorkspace(t, "tenant-a", "owner", "s1", "ws-1", "gen-7")
  got, err := svc.GetOrBind(context.Background(), craft.Scope{TenantID: 7, UserID: "owner", SessionID: "s1"})
  require.NoError(t, err)
  require.Equal(t, mobileworkspace.Binding{SessionID:"s1", WorkspaceID:"ws-1", Revision:1, Generation:"gen-7"}, got)
  require.Equal(t, 1, svc.ResolveCalls())
}
func TestMobileWorkspaceBindingRejectsBorrowedOwner(t *testing.T) {
  res := requestBinding(t, handlerForOwner("owner"), 7, "member", "s1")
  require.Equal(t, http.StatusForbidden, res.Code)
}
```

- [ ] **Step 2: Run the RED server tests.**

Run: `go test ./internal/application/service/mobileworkspace ./internal/handler/session -run 'Test(BindingServiceReusesOwnerWorkspace|MobileWorkspaceBindingRejectsBorrowedOwner)' -count=1`
Expected: FAIL because `BindingService` and the route handler do not exist.

- [ ] **Step 3: Write the mobile RED tests.**

```ts
it("drops a delayed binding from an old backend/account/tenant/generation scope", async () => {
  const api = deferredBindingClient();
  const state = createWorkspaceBindingStore(api);
  const old = state.open("s1", scope("https://a", "u1", "t1", "g1"));
  await state.open("s2", scope("https://b", "u2", "t2", "g2"));
  api.resolve(old, { session_id: "s1", workspace_id: "ws-old", revision: 1, generation: "g1" });
  expect(state.current()).toMatchObject({ sessionId: "s2", workspaceId: "ws-new" });
});
```

- [ ] **Step 4: Implement the minimal server and mobile adapters.**

```go
func (s *BindingService) GetOrBind(ctx context.Context, scope craft.Scope) (Binding, error) {
  ws, err := s.workspaces.Resolve(ctx, scope)
  if err != nil { return Binding{}, err }
  return Binding{SessionID: scope.SessionID, WorkspaceID: ws.ID, Revision: ws.Revision, Generation: ws.Generation}, nil
}
```

```ts
export class CloudWorkspaceClient {
  async getBinding({ sessionId, scope, signal }: GetBindingInput): Promise<WorkspaceBinding> {
    const body = await this.http.get(`/api/v1/sessions/${encodeURIComponent(sessionId)}/mobile-workspace`, { signal, scope });
    return parseWorkspaceBinding(body, sessionId, scope.generation);
  }
}
```

- [ ] **Step 5: Run GREEN and integration gates.**

Run: `go test ./internal/application/service/mobileworkspace ./internal/handler/session -run 'Test(BindingService|MobileWorkspaceBinding)' -count=1`
Run: `cd apps/mobile-next && npm test -- --runInBand tests/features/workspace-binding && npm run typecheck && npm run check:isolation`
Expected: owner reuse, tenant/owner denial, stale-response discard, and mobile compilation pass.

- [ ] **Step 6: Review and demo gate.** Verify an authenticated owner opens the same session twice and receives one workspace ID; verify a same-tenant non-owner gets 403. Record this as contract evidence only: no browser/device or provider-resume claim is produced.

## Execution Handoff

Implement in one fresh context. The fixed controller owns the later route mount, container construction, package export, and any migration reconciliation; this track supplies only the owned feature/service/handler files and its DI declaration.
