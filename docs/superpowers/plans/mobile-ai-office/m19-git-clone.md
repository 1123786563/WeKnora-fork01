# M19 Owner Git Connection Clone Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development`. Steps use checkbox syntax.

**Goal:** Let a Task Owner clone one repository into the Task’s bound cloud workspace using an active personal Git Connection.

**Architecture:** M09 owns the persistent workspace binding and M13 owns Run/PTY exclusion. This slice validates an owner-scoped AppConnector connection before clone, keeps credentials out of artifacts/chat, persists a repository binding, and supplies a narrow mobile clone form. Shared HTTP/DI/migration assembly remains controller-owned.

**Tech Stack:** Go/Gin/GORM, AppConnector authorization, Workbench, Expo 55/RN 0.83, Jest.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; Parent #3.

## Global Constraints

- Blocked by M09 WorkspaceBind and M13 RunPTY. Task is sessionId; scope is backend/accountId/tenantId/generation.
- GitHub is the first selected code-platform provider. `GitProvider`, `GitCredentialBroker`, and `SandboxGitPort` are proposed capabilities, not current AppConnector support. Only the Task Owner’s active GitHub Connection may clone; short-lived credential handles are injected into sandbox execution memory and redacted before logs/artifacts/mobile state.
- Clone checks authorization, workspace tenancy, and stopped/no-write-lease status before side effect. It is not an approval interaction; clone only starts after those checks.
- Allow only the selected GitHub Connection record; its Git execution capability is proposed here, not already supported by AppConnector. Do not promise multiple providers.
- Controller serially owns shared router/container/exports/lock/root layout/migration numbering; feature files stay beneath `apps/mobile-next/src/features/git-clone`.

## Review Focus

1. Viewer and non-owner Connection use is rejected before a provider call.
2. Revoked/inactive Connection, foreign workspace, and active Run/PTY lease cannot clone.
3. A request-id replay reconciles an existing clone operation rather than issuing another provider call.
4. Repository URL and branch validation reject shell-like arguments; no credentials enter display text.
5. Clone output identifies a bound repository state without granting remote push permission.

### Task M19: owner connection and clone

**Files:**
- Create: `internal/application/service/workbench/git_clone.go`, `internal/application/service/workbench/git_clone_test.go`.
- Create: `internal/application/repository/workbench_git_clone.go`, `internal/handler/session/workbench_git_clone.go`.
- Create: `apps/mobile-next/src/features/git-clone/cloneRepository.ts`, `apps/mobile-next/tests/features/git-clone.test.tsx`.
- Controller only: route registration, DI, exports, migration, root layout, manifests and lockfile.

**Interfaces:**
- Consumes: M09 `ResolveWorkspace(ctx, tenantID uint64, sessionID string) (WorkspaceBinding, error)`; M13 `RequireWorkspaceWriteIdle(ctx, tenantID uint64, sessionID string, expectedVersion int64) error`; current AppConnector connection reader.
- Produces (proposed): `CloneRepository(ctx context.Context, scope GitScope, in CloneRepositoryRequest) (GitRepositoryView, error)`.
- Proposed request: `CloneRepositoryRequest{RequestID, SessionID, ConnectionID, RepositoryURL, Branch string; ExpectedVersion int64}`. View: `RepositoryID`, `SessionID`, `RemoteURL`, `HeadCommit`, `State`, `Revision`.
- Proposed single execution chain: connection reader -> `GitCredentialBroker.Issue` -> `SandboxGitPort.Clone`; `GitProvider` is the GitHub remote adapter used by SandboxGitPort for clone/ref/push/PR queries, never a second clone caller. `GitCredentialBroker.Issue(ctx context.Context, tenantID uint64, accountID, connectionID string, authVersion int64) (CredentialHandle,error)`; `SandboxGitPort.Clone(ctx context.Context, workspace WorkspaceBinding, credential CredentialHandle, argv []string) (GitCloneReceipt,error)`; GitProvider internally provides `QueryRef/Push/CreatePR/QueryPR`. `argv` is validated structured arguments, never a shell string; handle is destroyed after call and redacted by adapter.

- [ ] **Step 1: Write RED tests.**

```go
func TestCloneRequiresOwnersActivePersonalConnection(t *testing.T) {
    require.ErrorIs(t, cloneErr(t, viewerScope, ownerConnection, request), ErrForbidden)
    require.ErrorIs(t, cloneErr(t, ownerScope, revokedConnection, request), ErrConnectionUnavailable)
    got := clone(t, ownerScope, ownerConnection, request); require.Equal(t, sessionID, got.SessionID)
}
```

```ts
it("never renders a credential and sends the chosen connection id", async () => {
  await submitClone({ connectionId: "conn-owner", repositoryUrl: "https://host/o/r.git" });
  expect(api.cloneRepository).toHaveBeenCalledWith(expect.objectContaining({ connection_id: "conn-owner" }));
});
```

- [ ] **Step 2: Run RED.**

Run: `go test ./internal/application/service/workbench -run 'TestClone' -count=1`.

Run: `cd apps/mobile-next && npm test -- --runInBand git-clone.test.tsx`.

Expected: fail because clone behavior is absent; do not skip provider authorization paths.

- [ ] **Step 3: Implement server admission and reconciliation.**

```go
func (s *GitService) CloneRepository(ctx context.Context, scope GitScope, in CloneRepositoryRequest) (GitRepositoryView, error) {
    if err := s.connections.RequireOwnerActive(ctx, scope.TenantID, scope.AccountID, in.ConnectionID); err != nil { return GitRepositoryView{}, err }
    if err := s.idle.RequireWorkspaceWriteIdle(ctx, scope.TenantID, in.SessionID, in.ExpectedVersion); err != nil { return GitRepositoryView{}, err }
    return s.clones.CreateOrReconcile(ctx, scope, in)
}
```

`CreateOrReconcile(ctx, scope, in) (GitRepositoryView,error)` first looks up unique `(tenant_id,owner_id,request_id)`; it returns completed row, returns in-progress state, or inserts a `queued` clone row and outbox record. Worker fixture `fakeSandboxGitPort{cloneCalls int; receipt GitCloneReceipt; err error}` asserts one call and no handle in receipt/log fields. Worker rechecks owner/active/AuthVersion, invokes the single broker->sandbox chain, and persists sanitized repository identity/head/state.

- [ ] **Step 4: Add handler/mobile view.**

Derive scope server-side, use actual filtered `connections()` data, require URL/branch confirmation, show only sanitized result/state, and disable duplicate submission while the request key is pending.

- [ ] **Step 5: GREEN, negatives, demo, handoff.**

Run: `go test ./internal/application/service/workbench -run 'TestClone' -count=1`.

Run: `cd apps/mobile-next && npm run typecheck && npm test -- --runInBand git-clone.test.tsx`.

Verify viewer/revoked/foreign-session/active-Run/replay cases and hand controller exact wiring/migration requirements.

## Review Gate

Reject client-provided tenant/owner scope, raw secret propagation, multi-provider promises unsupported by AppConnector, or any clone while M13 says write is active.
