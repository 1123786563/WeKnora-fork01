# T63 Task 5 R3 Implementation Report

- Task: terminalize pending intents only for a confirmed Agent-use denial; preserve pending work on transient gate errors.
- Worktree: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep/.worktrees/issue30-b6-coordination/.worktrees/issue30-b6-t63`
- Branch: `codex/issue30-t63`
- Base HEAD: `7e3a9534e172d69e24407c60b37c9eb43e61bbbc` (R2 checkpoint).
- Scope: `internal/modules/workbench/service/workbench/admission.go`, `admission_agent_use_test.go`, and this report.

## Changes

- All early Agent-use gate error branches now call pending settlement only when `errors.Is(err, ErrAgentUseDenied)` is true. Transient or other gate errors return unchanged without modifying request state or releasing its reservation.
- Added a pending request test with a stored reservation. First gate call returns a transient error and leaves the request pending with zero releases. Second call returns a wrapped retirement sentinel, which rejects the request and releases once. Existing pending retry, committed replay, and final-boundary race cases remain covered.

## Evidence

1. RED:
   - Command: `go test ./internal/modules/workbench/service/workbench/ -run TestAdmissionTransientAgentGateErrorKeepsPendingForRetry -count=1`
   - Result before implementation: FAIL; the request became `rejected` after the transient error, expected `pending`.
2. GREEN:
   - Command: `go test ./internal/modules/workbench/service/workbench/ -run 'TestAdmission(TransientAgentGateErrorKeepsPendingForRetry|ReplaysCommittedRunAfterRetirementButGatesPendingRetry|RetirementBetweenFastGateAndRunCommitIsDenied)' -count=1`
   - Result: PASS (`ok .../service/workbench 2.944s`).
3. Related suites:
   - `go test ./internal/modules/workbench/service/workbench/ -run 'TestAdmission|TestQueueNext' -count=1` — PASS (`8.722s`).
   - `go test ./internal/application/repository/ -run 'TestAgentRunAdmit|TestRetiredVariantAgentExists|TestWorkbenchRequest' -count=1` — PASS (`2.828s`).
   - `go test ./internal/handler/session/ -run 'TestQueueNextAgentUseDenied|TestWorkbench(StartHTTPIntegrationAndIdentityIsolation|StopThenRestartHTTPIntegration|QueueNextStaleRevisionIsAConflict)' -count=1` — PASS (`3.071s`).
   - `go test ./internal/container/ -run 'TestWorkbenchAdmission|TestAgentMarketplaceLifecycle' -count=1` — PASS (`2.981s`; linker warned about duplicate `-lc++`).
4. Build:
   - `go build ./...` — PASS, exit 0. Desktop/server link steps emitted the existing duplicate `-lc++` warning.
5. Diff:
   - `git diff --check` — PASS, no output.

## Limits

- The code path distinguishes errors via the shared sentinel and preserves wrapping through `errors.Is`; no live PostgreSQL run was performed.
- No repository, session handler, container, or T64-owned files were changed.

## Commit

- Local implementation commit: `fix(workbench): preserve pending intent on transient agent gate errors` (final HEAD is recorded by Git).
