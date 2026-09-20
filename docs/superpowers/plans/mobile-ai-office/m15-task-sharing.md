# M15 Task Sharing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task. Track checkboxes as work proceeds.

**Goal:** Let a Task owner grant and revoke read-only task visibility to members of the current space while preserving owner-only Run, terminal, approval, and write authority.

**Architecture:** Reuse existing session/share authorization vocabulary where compatible, but create a narrow Task ACL projection keyed by the existing session ID. Every Workbench/Run API checks the ACL on each request; the mobile client carries no role assertion and projects server capability flags. This slice changes read policy only and never gives a shared member a workspace writer lease.

**Tech Stack:** Go/GORM/Gin, existing session sharing authorization, Expo/TypeScript/Jest.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; design `docs/superpowers/specs/2026-09-20-mobile-ai-office-design.md`.

## Global Constraints

- M05 is required; Task is still the existing session ID and owner remains the sole writer.
- Shares only target members in the Task’s current tenant/space; no cross-tenant or public-link share exists.
- ACL is evaluated on every read and command; workspace IDs, cached mobile role, and prior SSE frames do not confer authority.
- Controller serially adds route/container/index/migration integration; this track owns its task-sharing feature/service/handler/test files and DI declaration.

## Review Focus

- A member can read allowed conversation/Run projections but receives 403 for continue, terminal, approval, and mutation commands.
- Revocation takes effect on the next request and causes mobile cached content to be removed by its existing scope/revocation hook.
- Owner cannot add a member from another tenant even if user ID exists globally.
- Duplicate grants/revokes are idempotent and preserve a monotonically changing ACL revision.
- Sharing original Task access does not alter an already published version’s ACL.

---

### Task 1: Current-space read-only task sharing vertical slice

**Files:**
- Create: `internal/application/service/mobileworkspace/task_sharing.go`
- Create: `internal/application/service/mobileworkspace/task_sharing_test.go`
- Create: `internal/application/repository/mobile_task_share.go`
- Create: `internal/application/repository/mobile_task_share_test.go`
- Create: `internal/handler/session/mobile_task_share.go`
- Create: `internal/handler/session/mobile_task_share_test.go`
- Create: `apps/mobile-next/src/features/task-sharing/TaskSharingClient.ts`
- Create: `apps/mobile-next/tests/features/task-sharing/TaskSharingClient.test.ts`
- Create: `apps/mobile-next/src/features/task-sharing/TaskCapabilityState.ts`
- Create: `apps/mobile-next/tests/features/task-sharing/TaskCapabilityState.test.ts`
- Modify/Test: `internal/router/routes_workbench.go:30-47,122-166`, `internal/handler/session/stream.go:35,219`, `internal/handler/session/workbench_read.go`, `internal/handler/session/craft_interaction.go`, `internal/handler/session/sandbox_terminal_bridge.go`, and `internal/handler/session/sandbox_terminal_ws.go` with their existing tests.

**Interfaces:**
- Consumes: M05 session/member authorization, existing `craft.Scope`, M09 owner binding (for owner-only write consumers), and existing Workbench session/Run query seams.
- Produces: **proposed** `TaskRole = "owner"|"reader"`; `TaskAccess.AuthorizeRead(ctx,tenantID,sessionID,userID) error` is called by Workbench read/snapshot/SSE, interactions, terminal ticket and WS; `TaskAccess.RequireOwnerWrite(...) error` gates continue/start/stop/decision/terminal input/mutations. `TaskGrant{SessionID string,MemberID string,Revision int64}`; `GrantReader(ctx, owner craft.Scope, memberID, requestID string, expectedRevision int64) (TaskGrant,error)`, `RevokeReader(...) error`, `Capabilities(ctx, scope craft.Scope) (TaskCapabilities,error)`; **proposed** share/revoke/capabilities endpoints under `/sessions/:session_id/mobile-workspace/share`.

- [ ] **Step 1: Write ACL and handler RED tests.**

```go
func TestTaskReaderCanReadButCannotContinueTerminalOrApprove(t *testing.T) {
  h := sharedTaskHandler(t, grant("s1", "member"))
  require.Equal(t, 200, getWorkspaceSnapshot(t,h,"member","s1").Code)
  require.Equal(t, 403, postContinue(t,h,"member","s1").Code)
  require.Equal(t, 403, openPTY(t,h,"member","s1").Code)
  require.Equal(t, 403, postDecision(t,h,"member","s1").Code)
}
func TestTaskShareRejectsMemberOutsideCurrentTenant(t *testing.T) {
  _, err := sharingService(t).GrantReader(ctx, ownerScope("s1"), "foreign-user", "g1", 1)
  require.ErrorIs(t, err, craft.ErrForbidden)
}
```

- [ ] **Step 2: Run RED.**

Run: `go test ./internal/application/repository ./internal/application/service/mobileworkspace ./internal/handler/session -run 'Test(TaskReaderCanReadButCannotContinueTerminalOrApprove|TaskShareRejectsMemberOutsideCurrentTenant)' -count=1`
Expected: FAIL because the narrow Task ACL and command guards do not exist.

- [ ] **Step 3: Write mobile RED tests.**

```ts
it("removes execution affordances for a reader from server capabilities", () => {
  expect(projectTaskCapabilities({ role:"reader", can_read:true, can_run:false, can_terminal:false, can_approve:false, can_write:false }))
    .toMatchObject({ showConversation:true, showRun:true, showExecute:false, showTerminalInput:false, showApproval:false });
});
it("clears task cache after revoked capability response", () => expect(revokeCache("s1")).toEqual(["task:s1"]));
```

- [ ] **Step 4: Implement server-enforced roles.**

```go
func (s *TaskSharing) Capabilities(ctx context.Context, scope craft.Scope) (TaskCapabilities,error) {
  if s.owner(ctx,scope) { return ownerCapabilities(),nil }
  if s.grants.ActiveReader(ctx,scope.TenantID,scope.SessionID,scope.UserID) { return readerCapabilities(),nil }
  return TaskCapabilities{}, craft.ErrNotFound
}
func requireTaskWrite(c TaskCapabilities) error { if !c.CanRun || !c.CanWrite { return craft.ErrForbidden }; return nil }
```

```ts
export const projectTaskCapabilities = (c: TaskCapabilities) => ({ showConversation:c.can_read, showRun:c.can_read, showExecute:c.can_run, showTerminalInput:c.can_terminal, showApproval:c.can_approve });
```

- [ ] **Step 5: Run GREEN gates.**

Run: `go test ./internal/application/repository ./internal/application/service/mobileworkspace ./internal/handler/session -run 'Test(TaskShare|TaskReader|MobileTaskShare)' -count=1`
Run: `cd apps/mobile-next && npm test -- --runInBand tests/features/task-sharing && npm run typecheck && npm run check:isolation`
Expected: current-space grant/revoke, command denial, and reader-only mobile projection pass.

- [ ] **Step 6: Review and demo gate.** Owner grants a current-space member; member reads conversation/Run then receives server 403 for continue, PTY, approval, and file mutation. Owner revokes; next fetch is invisible and local task cache is cleared. Do not claim published-copy behavior is changed.

## Execution Handoff

Use one fresh context. Preserve existing share behavior outside the mobile Task boundary and do not implement publication or shared-resource endpoints; M16 consumes this reader ACL.
