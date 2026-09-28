# T63 Task 5 R2 Implementation Report

- Task: settle same-hash pending Agent admission intents when the early lifecycle gate denies.
- Worktree: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep/.worktrees/issue30-b6-coordination/.worktrees/issue30-b6-t63`
- Branch: `codex/issue30-t63`
- Base HEAD: `b4ac07f318d7111ba2f427460e3be81f7ddbf41d` (R1 checkpoint).
- Scope: `internal/modules/workbench/service/workbench/admission.go`, `admission_agent_use_test.go`, and minimal `internal/application/repository/workbench_request.go` CAS support, plus this report.

## Changes

- Early gate denials now reconcile the durable request again, covering both the existing-request path and a request that appeared after the initial miss / unique-insert race.
- A same-hash pending intent is rejected only after a transaction takes the same tenant/session write lock as `AgentRunStore.Admit`, verifies no AgentRun exists for the request, and wins a pending-to-rejected CAS. The stored reservation reference is retained; its unstarted reservation is released only by the caller that wins the CAS, and empty references are not released.
- If a concurrent AgentRun already committed while its workbench intent remains pending, reconciliation resumes it rather than rejecting or releasing it. Existing admitted/dispatching replay and request-hash conflict behavior remain intact.
- Replaced the previous test expectation that a denied pending intent stays pending. Added assertions for rejected state, exactly-once release across retries, no run/session slot, and preservation of an already-committed Run.

## Evidence

1. RED:
   - Command: `go test ./internal/modules/workbench/service/workbench/ -run 'TestAdmissionReplaysCommittedRunAfterRetirementButGatesPendingRetry' -count=1`
   - Result before implementation: FAIL; the pending intent remained `pending` instead of transitioning to `rejected`.
2. GREEN and repetition:
   - `go test ./internal/modules/workbench/service/workbench/ -run 'TestAdmission(ReplaysCommittedRunAfterRetirementButGatesPendingRetry|RetirementBetweenFastGateAndRunCommitIsDenied)' -count=5` — PASS (`ok .../service/workbench 11.335s`).
3. Related suites:
   - `go test ./internal/modules/workbench/service/workbench/ -run 'TestAdmission|TestQueueNext' -count=1` — PASS (`8.116s`).
   - `go test ./internal/application/repository/ -run 'TestAgentRunAdmit|TestRetiredVariantAgentExists|TestWorkbenchRequest' -count=1` — PASS (`3.148s`).
   - `go test ./internal/handler/session/ -run 'TestQueueNextAgentUseDenied|TestWorkbench(StartHTTPIntegrationAndIdentityIsolation|StopThenRestartHTTPIntegration|QueueNextStaleRevisionIsAConflict)' -count=1` — PASS (`2.557s`).
   - `go test ./internal/container/ -run 'TestWorkbenchAdmission|TestAgentMarketplaceLifecycle' -count=1` — PASS (`1.869s`; linker warned about duplicate `-lc++`).
4. Build:
   - `go build ./...` — PASS, exit 0. Desktop/server link steps emitted the existing duplicate `-lc++` warning.
5. Diff:
   - `git diff --check` — PASS, no output.

## Limits

- Concurrency coordination uses transaction/session row locking exercised by the existing SQLite fixture. No live PostgreSQL runtime test was performed.
- Task 6 and T64-owned files were not changed.

## Commit

- Local implementation commit: `fix(workbench): settle pending retired-agent retries` (final HEAD is recorded by Git).
