# T01 RunView R2 executor binding report

**Status:** scoped implementation is focused-GREEN and ready for independent review. No commit created. Production assembly remains a separate R5 dependency.

## Change

Added the typed `RunSessionResolver` / `RunSessionBinding` contract and `NewRunBoundExecutor`. Every `Execute`, `Observe`, and `Abort` resolves a binding from the Task's persisted scope and Run fence and validates the exact tenant/owner/session/Run key, lease fence completeness, pinned session ID shape, and equality between the binding directory and the immutable directory on its `Client`.

The executor passes that per-call Client/session through message snapshot reads, the global event subscription, prompt submission, abort, and final snapshot verification. It still checks Task/workspace ownership through `boundWorkspace`, but does not read `workspace.OpenCodeSessionID`. The historical `NewExecutor(client, store, emit)` signature remains source-compatible and intentionally fails closed because a Task-level Client cannot select a RunView. No prompt, event-stream, status, message, or abort request is made without a resolver and valid binding.

Focused tests cover two Runs sharing a Craft Task scope/workspace but using distinct sessions/directories across Execute, Observe, and Abort; invalid/mismatched bindings; legacy workspace session absence; and missing resolver fail-closed behavior. The runtime fake rejects session IDs other than its own and records session and directory for each request. Existing persisted-message retry tests were migrated to an explicit resolver and continue asserting no duplicate prompt submission.

## RED → GREEN and verification

- RED: `go test ./internal/modules/agentruntime/agent/opencode -run 'TestRunBoundExecutor' -count=1` failed at compile time because `RunSessionBinding` and `NewRunBoundExecutor` did not exist.
- GREEN focused/full package: `go test ./internal/modules/agentruntime/agent/opencode -count=1` — PASS (`34.672s`).
- Concurrency verification: `go test -race ./internal/modules/agentruntime/agent/opencode -run 'TestRunBoundExecutor|TestObserveReadsSnapshotOnly|TestAbortRecordsCanceledOnlyWhenSnapshotConfirms|TestExecuteUsesRunBindingAndRejectsForbiddenWorkspaceScope|TestUnknownAcceptanceRemoteAcceptedReadTimeoutNoSecondPost' -count=1 -timeout=90s` — PASS (`4.873s`).
- Formatting: `gofmt -w` on all four owned Go files — PASS.
- Whitespace: `git diff --check -- internal/modules/agentruntime/agent/opencode/executor.go internal/modules/agentruntime/agent/opencode/executor_test.go internal/modules/agentruntime/agent/opencode/unknown_acceptance_test.go` — PASS.

The full package run excludes live protocol execution unless `CRAFT_LIVE=1`; no live image or Linux isolation proof is claimed.

## Exact checkpoint

Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`.
Starting and checkpoint HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (no commit made).

This was resumed work. At resume, `executor_test.go` already contained an uncommitted runtime-fake staging scaffold from the preceding TDD turn; its resume-time content hash was not recorded. The complete final owned-file delta against unchanged HEAD, including that scaffold and the newly added resolver behavior tests, is preserved in `2026-09-24-craft-107-runview-r2-executor-binding-checkpoint.patch`. Final full-content SHA-256 values are recorded in the adjacent checkpoint manifest. Other shared-worktree changes were left untouched.

## Scope and remaining integration dependency

Changed source/tests are limited to:

- `internal/modules/agentruntime/agent/opencode/executor.go`
- `internal/modules/agentruntime/agent/opencode/executor_test.go`
- `internal/modules/agentruntime/agent/opencode/unknown_acceptance_test.go`
- `internal/modules/agentruntime/agent/opencode/run_binding.go` (new)

R5 must provide and inject the production resolver through `NewRunBoundExecutor`. Existing production callers that still construct this executor using `NewExecutor` will now fail closed by design; this checkpoint does not claim the service/container assembly is wired. Per-Run process/container isolation and the Linux image gate remain separate requirements. The OpenCode `/event` endpoint is global to its server, so this work does not establish OS-level or cross-Run event-process isolation.
