# Issue 140 T14 Backend Review Fix R1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make concurrent Workbench application-task replay return the original link or typed conflict, canonicalize application IDs, and prove the real owner-scoped read/archive path.

**Architecture:** Keep the Workbench-owned projection and narrow Career interface. Wrap the existing transaction in a small bounded retry for concurrent request-key creation races; the retry must re-read the committed mapping and reuse the existing replay/conflict logic. Do not weaken database uniqueness or convert unrelated database failures into conflicts.

**Tech Stack:** Go/GORM, SQLite concurrency tests, existing Workbench repository authorization seams.

**Spec:** `docs/plans/2026-09-24-issue-140-t14-backend.md`; review package `review-6d67b51ae..c67cec29d.diff`; independent review findings in the T14 Task 1 review.

## Global Constraints

- Work only in `/Users/wuyongjun/.codex/worktrees/issue-140-t14-backend/WeKnora-fork01`.
- Fix BASE is `c67cec29d26ff911e4254d4a2034ff390a4ebd73`; local commit authorized, no push/merge.
- Do not alter Career files, migration schema, route manifests, or unrelated Workbench behavior.
- Concurrent exact replay must return the same Task/Run; concurrent changed intent must return `ErrApplicationTaskConflict`; neither may leak a raw unique/lock error.
- Retry only evidence of concurrent creation (`gorm.ErrDuplicatedKey`, the two request uniqueness constraint names, or SQLite database-lock contention), with a small bounded attempt count. Other errors remain original errors.
- Canonicalize accepted UUID spellings to `uuid.UUID.String()`.
- Exercise `AgentRunStore.GetOwnedRun`, `WorkbenchListStore.ReadTaskFactsForRun`, and cross-scope archive denial; no mock may replace these repository calls.

## Review Focus

- A loser after `agent_runs.uq_agent_runs_request` must roll back and reconcile the committed winner.
- A changed application/title racing under the same request must become a typed conflict after re-read.
- Braced/raw/URN UUID forms must replay as the same canonical application ID.
- Cross-tenant/owner run read, task facts read, and archive must all deny without mutation.

---

### Task 1: Concurrent Replay And Authorization Evidence

**Files:** modify `internal/modules/workbench/service/workbench/application_task.go` and `application_task_test.go` only.

**Interfaces:** preserve `ApplicationTaskCoordinator.EnsureCareerApplicationTask` and `FindCareerApplicationTask` signatures exactly.

- [ ] **RED:** add tests:

```go
func TestEnsureCareerApplicationTaskConcurrentExactReplayReturnsOriginal(t *testing.T)
func TestEnsureCareerApplicationTaskConcurrentChangedIntentIsTypedConflict(t *testing.T)
func TestEnsureCareerApplicationTaskCanonicalizesEquivalentUUIDForms(t *testing.T)
```

Extend `TestApplicationTaskProjectionIsListReadableArchivableAndRestorable` to use the real `AgentRunStore.GetOwnedRun` and `WorkbenchListStore.ReadTaskFactsForRun`, assert cross-tenant and cross-owner `ErrNotFound`, and assert cross-scope `SetTaskArchived` returns `ErrWorkbenchTaskNotFound` before valid archive/restore. Concurrent tests must synchronize goroutine starts and reject any non-typed database error.

- [ ] **GREEN:** extract the current transaction body into an unexported `ensureOnce`; normalize the parsed UUID with `parsed.String()`. `EnsureCareerApplicationTask` calls `ensureOnce` up to three times. Retry only when the rolled-back error is a duplicate/lock race attributable to concurrent request creation, with a short pause between attempts. On retry, existing mapping replay returns the original link or typed conflict. Preserve all non-race errors.

- [ ] **VERIFY:**

```bash
gofmt -w internal/modules/workbench/service/workbench/application_task.go internal/modules/workbench/service/workbench/application_task_test.go
go test ./internal/modules/workbench/service/workbench -run 'Test.*ApplicationTask' -count=1
go test ./internal/modules/workbench/service/workbench -count=1
go test ./internal/application/repository -run 'TestWorkbench.*Task|TestWorkbenchList|TestReadTaskFacts' -count=1
git diff --check
```

- [ ] **COMMIT:** append the exact RED/GREEN evidence to the existing Task 1 report, then commit:

```bash
git add internal/modules/workbench/service/workbench/application_task.go internal/modules/workbench/service/workbench/application_task_test.go
git commit -m 'fix(workbench): reconcile concurrent application task creation'
```
