# M22 Git PR Approval and Reconciliation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development`. Check off each step.

**Goal:** Create a pull request only after a separate, exact approval and reconcile unknown creation without creating duplicates; merging remains external.

**Architecture:** M21 provides a reconciled pushed head. This slice persists a separate PR operation with base/head/repo/summary hash, invokes the selected provider once after approval, then reconciles by provider lookup. Mobile offers the proposed PR and records provider URL/state. Controller owns shared assembly.

**Tech Stack:** Go/Gin/GORM, approval durability, selected AppConnector provider, Expo/Jest.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; Parent #3.

## Global Constraints

- Blocked by M21. Scope backend/accountId/tenantId/generation; Task=sessionId.
- PR approval is distinct from push and binds repo, base, head, exact head commit, title/body summary, credential version, expected version and hash.
- Unknown creates `waiting_user`; reconciliation queries provider for the exact base/head before any user-authorized retry. Merge is external to WeKnora.
- Owner-only Connection; do not expose provider token or claim platform-neutral support beyond installed selected provider.
- Controller owns routes/container/exports/lock/layout/migration numbering.

## Review Focus

1. Base/head or title change requires a new approval.
2. Existing provider PR resolves unknown state without duplicate POST.
3. Viewer cannot request, approve, or reconcile.
4. Merge result is displayed as external status, never a server-side merge command.
5. Idempotent request maps to one operation.

### Task M22: separately approved PR

**Files:**
- Create: `internal/application/service/workbench/git_pr.go`, `internal/application/service/workbench/git_pr_worker.go`, `internal/application/service/workbench/git_pr_test.go`, `internal/application/repository/git_pr_operation.go`, `internal/application/repository/git_pr_outbox.go`.
- Create: `internal/handler/session/workbench_git_pr.go`, `apps/mobile-next/src/features/git-pr/gitPr.ts`, `apps/mobile-next/tests/features/git-pr.test.tsx`.
- Controller only: shared wiring/migration/root/export/lock.

**Interfaces:**
- Consumes: M21 `GitRemoteOperationView{State,RemoteHead,RepositoryID}`.
- Produces (proposed): `RequestPullRequest(ctx, GitScope, PullRequestInput) (GitPROperationView,error)` and `ReconcilePullRequest(ctx, GitScope, operationID string) (GitPROperationView,error)`.
- Input: `PullRequestInput{RequestID,SessionID,RepositoryID,Base,Head,HeadCommit,Title,Body string; ExpectedVersion int64}`.
- Proposed lifecycle: `draft -> awaiting_approval -> queued -> sending -> succeeded|waiting_user|failed`, unique `(tenant_id,owner_id,request_id)`, approval interaction/hash/AuthVersion and CAS-leased outbox. `ClaimPR(ctx,now)`, `MarkPRSent(ctx,id,lease,providerID,url)`, `MarkPRUnknown(ctx,id,lease,cause)`, `QueryPR(ctx, repo, base, head, headCommit)`. Query accepts exactly one open provider PR whose repo/base/head/head SHA match; zero stays waiting_user and more than one is an explicit ambiguous failure.

- [ ] **Step 1: RED.**

```go
func TestUnknownPRIsReconciledByExactBaseAndHead(t *testing.T) {
 op := createApprovedPR(t, store, owner, pr); provider.CreateUnknown = true; worker.RunOnce(ctx); reconcilePR(t, svc, op.ID)
 require.Equal(t, 1, provider.CreateCalls); require.Equal(t, "https://provider/pr/7", operation(t,op).URL)
}
```

- [ ] **Step 2: Run RED.** `go test ./internal/application/service/workbench -run 'Test.*PR' -count=1`; `cd apps/mobile-next && npm test -- --runInBand git-pr.test.tsx` must fail before implementation.

- [ ] **Step 3: Implement.** Persist hash-bound operation and outbox in one transaction; worker CAS-claims, rechecks owner/AuthVersion, verifies M21 head/M06 approval, sends once, and marks unknown on lost receipt. Query exact repo/base/head/head SHA only; no merge method exists.

- [ ] **Step 4: Mobile.** Show base/head/commit/title before approval, provider URL after reconciliation, a waiting-user query control, and text that merging happens in the code platform.

- [ ] **Step 5: GREEN/handoff.** Repeat commands for changed-fields/viewer/replay/existing-PR tests and send serial assembly requirements to controller.

## Review Gate

Reject reuse of push approval, duplicate create after timeout, a merge API, or an unbounded provider abstraction.

## Demo and evidence

- [ ] Start with an M21 reconciled remote head and record exact base/head.
- [ ] Capture separate PR approval details and its parameter hash.
- [ ] Change title, base, head, commit, or credential revision; each must require a new approval.
- [ ] Use a safe provider test condition to produce unknown outcome only when available.
- [ ] Capture the exact-base/head reconciliation result and confirm provider create count is one.
- [ ] Show provider PR URL and the external merge statement in mobile.
- [ ] Capture viewer denial before durable operation creation.
- [ ] Classify this evidence as API/provider integration, not real-device acceptance.

## Controller assembly request

- [ ] Allocate migration number after current integrated migration scan.
- [ ] Register durable PR request/reconcile routes under existing Workbench guards.
- [ ] Wire only the selected existing AppConnector platform capability.
- [ ] Re-run service and mobile contract tests following serial integration.
