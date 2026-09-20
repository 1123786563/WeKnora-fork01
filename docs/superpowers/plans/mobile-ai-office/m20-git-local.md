# M20 Git Status, Diff, and Local Commit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development`. Steps use checkbox syntax.

**Goal:** Let the Owner inspect cloud-workspace Git status/diff and create a local commit without creating a remote side effect or approval.

**Architecture:** M19 supplies a bound sanitized repository. This slice provides read endpoints and a local CAS commit command; it makes no remote push/PR request. Mobile renders server-returned status/diff and exact new commit ID. Controller alone wires shared API/root/lock/migration files.

**Tech Stack:** Go/Gin/GORM, Workbench Git adapter, Expo 55/RN 0.83, Jest.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; Parent #3.

## Global Constraints

- Blocked by M19 (which carries M13 writer admission). Scope: backend/accountId/tenantId/generation; Task is sessionId.
- Status/diff/local commit require Owner and bound workspace. No approval is requested for these cloud-local operations.
- Commit uses expected version and request-id; conflicts return current repository snapshot and do not silently retry.
- Never call remote push/PR APIs, force push, or remote branch deletion in this ticket.
- Controller owns all shared assembly; feature files are `apps/mobile-next/src/features/git-local/**`.

## Review Focus

1. Viewer cannot read a private repository status or diff.
2. A stale expected version returns a conflict with current head rather than creating a commit.
3. Replayed request-id returns the prior local commit once.
4. Diff paths are rendered as data, never passed through a shell.
5. Local commit outcome cannot be shown as “pushed” or “PR created.”

### Task M20: local Git inspection and commit

**Files:**
- Create: `internal/application/service/workbench/git_local.go`, `internal/application/service/workbench/git_local_test.go`.
- Create: `internal/handler/session/workbench_git_local.go`.
- Create: `apps/mobile-next/src/features/git-local/gitLocal.ts`, `apps/mobile-next/tests/features/git-local.test.tsx`.
- Controller only: shared route/DI/export/root/migration/manifest/lock edits.

**Interfaces:**
- Consumes: M19 `GitRepositoryView{RepositoryID, SessionID, HeadCommit, Revision}` and M13 `RequireWorkspaceWriteIdle(ctx, tenantID, sessionID string, expectedVersion int64) error`.
- Produces (proposed port): `SandboxGitPort.Status/Diff/Commit(ctx, workspace, argv)`, where only `Commit` is writer-admitted; status/diff remain read-only.
- Produces (proposed): `GetGitStatus(ctx, scope GitScope, sessionID string) (GitStatusView,error)`, `GetGitDiff(ctx, scope GitScope, sessionID, base, head string) (GitDiffView,error)`, `CommitLocal(ctx, scope GitScope, in LocalCommitRequest) (GitCommitView,error)`.
- Proposed commit request: `LocalCommitRequest{RequestID, SessionID, Message string; ExpectedVersion int64}`.

- [ ] **Step 1: Write RED tests.**

```go
func TestLocalCommitUsesCASAndDoesNotRemoteWrite(t *testing.T) {
    got := commitLocal(t, ownerScope, LocalCommitRequest{RequestID: "m20", SessionID: sessionID, ExpectedVersion: 4})
    require.NotEmpty(t, got.CommitID); require.Zero(t, remote.pushCalls)
    require.ErrorIs(t, commitLocalErr(t, ownerScope, staleRequest), ErrVersionConflict)
}
```

```ts
it("labels a local commit with its exact server commit id", async () => {
  render(<GitLocalScreen state={committed} />); expect(screen.getByText(committed.commit_id)).toBeTruthy();
  expect(screen.queryByText(/pushed/i)).toBeNull();
});
```

- [ ] **Step 2: Run RED.**

Run: `go test ./internal/application/service/workbench -run 'Test(LocalCommit|GitStatus|GitDiff)' -count=1`.

Run: `cd apps/mobile-next && npm test -- --runInBand git-local.test.tsx`.

Expected: fail before status/diff/local-commit interfaces exist.

- [ ] **Step 3: Implement local-only behavior.**

```go
func (s *GitService) CommitLocal(ctx context.Context, scope GitScope, in LocalCommitRequest) (GitCommitView, error) {
    repo, err := s.repos.RequireOwnerRepository(ctx, scope, in.SessionID); if err != nil { return GitCommitView{}, err }
    if err := s.writer.RequireWorkspaceWriteIdle(ctx, scope.TenantID, in.SessionID, in.ExpectedVersion); err != nil { return GitCommitView{}, err }
    return s.local.CommitOnce(ctx, repo, in.RequestID, in.Message, in.ExpectedVersion)
}
```

Read operations return sanitized paths/diff bytes subject to server limits. Commit changes only the bound workspace and returns its new head/revision.

- [ ] **Step 4: Implement mobile contract.**

Fetch status/diff from server, send a user-entered commit message with idempotency/expected version, preserve form input on conflict, and navigate to M21 only after a committed state is refreshed.

- [ ] **Step 5: GREEN and handoff.**

Run: `go test ./internal/application/service/workbench -run 'Test(LocalCommit|GitStatus|GitDiff)' -count=1`.

Run: `cd apps/mobile-next && npm run typecheck && npm test -- --runInBand git-local.test.tsx`.

Verify owner/viewer/stale/replay/no-remote-call and active-Run/PTY-writer rejection cases; hand controller wiring requirements.

## Review Gate

Reject a design that reuses M21 approval for local commit, writes remote state, hides a CAS conflict, or passes diff text to a shell.
