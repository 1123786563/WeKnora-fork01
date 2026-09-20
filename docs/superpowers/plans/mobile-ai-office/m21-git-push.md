# M21 Exact Git Push Approval and Reconciliation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development`. Check off each step.

**Goal:** Require one exact approval for each remote branch push and hold unknown results for query-based reconciliation without repeating the push.

**Architecture:** M20 provides an exact local commit; M06 provides durable approval/CAS behavior. This slice stores an immutable proposed remote operation and parameter hash, requests approval, sends one provider push after approval, and persists `waiting_user` until a provider query resolves unknown outcome. Shared router/container/migration assembly is controller work.

**Tech Stack:** Go/Gin/GORM, existing approval durability, AppConnector, Expo 55/RN 0.83.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; Parent #3.

## Global Constraints

- Blocked by M06 Approval and M20 GitLocal; Task=sessionId, scope=backend/accountId/tenantId/generation.
- Approval binds `action=push`, repo, branch, exact commit, credential version, summary, expected version and parameter hash. Any change requires a new approval.
- Unknown provider outcome is `waiting_user`; query remote ref before any retry. No force push or remote deletion.
- Owner Connection only; viewer rejection occurs before creating approval or provider work.
- Controller serially owns root/router/container/exports/lock/layout/migration files.

## Review Focus

1. Changed branch, commit, repo, or credential version invalidates approval.
2. Timeout after send records unknown and query resolves it without a second push.
3. A remote ref mismatch remains blocked for user resolution.
4. Replay cannot create two approved remote writes.
5. Mobile displays exact commit/branch and waiting state, never an assumed success.

### Task M21: approved exact push

**Files:**
- Create: `internal/application/repository/git_remote_operation.go`, `internal/application/repository/git_remote_outbox.go`, `internal/application/service/workbench/git_push.go`, `internal/application/service/workbench/git_push_worker.go`, `internal/application/service/workbench/git_push_test.go`.
- Create: `internal/handler/session/workbench_git_push.go`, `apps/mobile-next/src/features/git-push/gitPush.ts`, `apps/mobile-next/tests/features/git-push.test.tsx`.
- Controller only: shared route/DI/migration/root/export/lock assembly.

**Interfaces:**
- Consumes: M06 `Decide(... argsHash string ...)`; M20 `GitCommitView{CommitID,Revision}`.
- Produces (proposed): `RequestPush(ctx, GitScope, PushRequest) (GitRemoteOperationView,error)` and `ReconcilePush(ctx, GitScope, operationID string) (GitRemoteOperationView,error)`.
- `PushRequest{RequestID,SessionID,RepositoryID,Branch,CommitID,Summary string; ExpectedVersion int64}`; view includes `OperationID, ArgsHash, CredentialVersion, State, RemoteHead`.
- Proposed state/CAS: `draft -> awaiting_approval -> queued -> sending -> succeeded|waiting_user|failed`; unique `(tenant_id, owner_id, request_id)`, `approval_interaction_id`, immutable hash/credential version, and transactional outbox claim. Worker `ClaimPush(ctx, now) (PushOperation,error)`, `MarkPushSent(ctx,id,lease,receipt) error`, `MarkPushUnknown(ctx,id,lease,cause) error`; `QueryPushRef(ctx, operation) (head string,error)` is the only unknown reconciliation action.

- [ ] **Step 1: RED tests.**

```go
func TestUnknownPushIsQueriedNotRetried(t *testing.T) {
 op := createApprovedPush(t, store, owner, exactPush); provider.failUnknownOnce = true; worker.RunOnce(ctx)
 require.Equal(t, WaitingUser, store.Get(t, op.ID).State); reconcilePush(t, svc, op.ID)
 require.Equal(t, 1, provider.PushCalls); require.Equal(t, exactPush.CommitID, operation(t, op).RemoteHead)
}
```

- [ ] **Step 2: Run RED.** Run `go test ./internal/application/service/workbench -run 'Test.*Push' -count=1`; then `cd apps/mobile-next && npm test -- --runInBand git-push.test.tsx`. Expected: missing push/reconcile behavior fails.

- [ ] **Step 3: Implement immutable operation and recovery.**

```go
func (s *GitService) RequestPush(ctx context.Context, scope GitScope, in PushRequest) (GitRemoteOperationView,error) {
 if err := s.repos.RequireOwnerCommit(ctx, scope, in); err != nil { return GitRemoteOperationView{},err }
 return s.remote.CreateApprovedOperation(ctx, scope, in, hashPush(in))
}
```

The executor claims one outbox row by CAS, rechecks owner/active AuthVersion and M13 writer admission, sends once only after M06 approval matches hash/credential revision, records provider receipt if available, and transitions unknown to `waiting_user`. `ReconcilePush` calls only `QueryPushRef`; mismatch remains waiting_user and never enqueues another send.

- [ ] **Step 4: Mobile behavior.** Render repo/branch/commit/summary before approval, refresh durable state after approval, offer “check remote result” only for `waiting_user`, and never auto-resubmit.

- [ ] **Step 5: GREEN/handoff.** Run the two RED commands plus owner/viewer/hash-change/credential-change/ref-mismatch/replay cases; send controller wiring request.

## Review Gate

Reject retry-on-timeout, mutable approval fields, force flags, or success inferred from network completion.

## Demo and evidence

- [ ] Start with one owner-selected repository whose M20 head is recorded.
- [ ] Capture the approval sheet showing repository, branch, commit, summary, hash and credential revision.
- [ ] Change one field and prove the old approval is rejected before dispatch.
- [ ] Inject the selected provider’s safe unknown-result test condition only when the environment supports it.
- [ ] Capture the durable `waiting_user` response and the subsequent query result.
- [ ] Compare provider push invocation count before and after reconciliation; it remains one.
- [ ] Capture a viewer request rejection and verify no operation record became approval-pending.
- [ ] Record this as API/contract evidence, not iOS/Android cloud acceptance.

## Controller assembly request

- [ ] Controller allocates the next versioned migration after scanning current migration layout and adds its matching SQLite migration using repository naming conventions.
- [ ] Register exact command/read endpoints behind existing Workbench authorization guards.
- [ ] Wire operation store and provider adapter in container only after reviewed track code lands.
- [ ] Re-run scoped tests after assembly and retain unknown-result state across restart.
