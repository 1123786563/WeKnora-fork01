# Craft #107 T19 Docker Send Coordinator S2 Task 1 Report

## Result

Implemented the server-owned coordinator boundary for the S2 restricted Docker send flow. Preparation commits the existing Task Budget hold and unresolved journal intent before any inert create; a pre-receipt replay may reuse that intent. After create, the exact persisted Docker receipt can be recovered with `ResumeBound`; the repository grants a process-local, single-use permission only to the single durable claim winner. A claimed operation reloads as observation-only. This task contains no Docker/provider transport call, and no real `ExecStart` occurred.

The S1 repository seam was extended with the parent-authorized current Run fence. Bind and claim now acquire the `agent_runs` write/row lock, check its current revision and executable status, and perform the journal CAS in the same database transaction. The allowed statuses follow the existing `chargeableCraftRun` contract exactly: `queued`, `running`, and `recovering`. This removes the earlier coordinator preflight TOCTOU window.

## Changed files

- `internal/application/service/craft_docker_send_coordinator.go` — prepare/reuse, bound receipt recovery, read-only observation, single-use send permission, and claim orchestration.
- `internal/application/service/craft_docker_send_coordinator_test.go` — hold-before-create, safe pre-receipt replay, bind/resume/claim, divergent receipt, one-owner concurrency, replay, cancellation, stale Run epoch, unresolved hold, and no-resolution-after-claim coverage.
- `internal/application/service/craft_budget.go` — narrow `validateDockerSendRun` preflight seam. This file already contained concurrent/shared uncommitted changes at task start; only this task's appended function belongs to S2.
- `internal/application/repository/craft_docker_send_claim.go` — parent-authorized S1 seam extension: current Run lock/revision/status validation inside bind/claim transactions.
- `internal/application/repository/craft_docker_send_claim_test.go` — current revision and paused-status rejection tests across the existing SQLite/PostgreSQL test harness.

No migration, `agent_run.go`, T01/T05, adapter, or container wiring files were changed. No commit was created.

## Verification evidence

Commands run from `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`:

| Command | Result |
| --- | --- |
| `go test ./internal/application/service -run '^TestCraftDockerSendCoordinator' -count=1` | PASS |
| `go test -race ./internal/application/service -run '^TestCraftDockerSendCoordinatorClaimIsOneOwnerAndReplayCannotResend$' -count=1` | PASS |
| `go test ./internal/application/repository -run '^TestCraftDockerSendClaim' -count=1` | PASS |
| `go test ./internal/application/service -run 'TestCraftChargeStart|TestCraftBudgetStart|TestCraftBudget.*(Start|Journal)|TestCraftT19Journey' -count=1` | PASS |
| `gofmt -d` on all five changed Go files | No output |
| `git diff --check` | No output |
| `git diff --no-index --check /dev/null` on the two new coordinator files | No whitespace diagnostics; exit 1 is the expected “files differ” status for added files |

SQLite CLI reports version `3.54.0`. The repository test harness ran SQLite cases. PostgreSQL was explicitly skipped because `TRPC_TEST_POSTGRES_DSN` is unset; PostgreSQL execution of the new transaction and lock path remains unverified here.

## Checkpoint

- Base HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`.
- Exact final file snapshots and SHA-256/byte counts: `docs/plans/2026-09-24-craft-107-t19-docker-send-coordinator-s2-task1-checkpoint.json` and its `snapshot/` directory.
- The coordinator source and test files were absent before this task. `craft_budget.go` had concurrent/shared edits before this task, so its pre-task hash was not captured; the report identifies this task's appended method. The repository claim files were the existing S1 untracked files; the S1 report/checkpoint remains their pre-S2 baseline evidence.

## Limits and assumptions

- Coordinator claim permission proves the durable one-owner CAS was consumed; the later transport task must call its physical send at most once after consuming that permission. The coordinator intentionally provides no provider callback.
- Once claimed, errors remain unresolved `intent` with the existing dispatched hold. There is no API to infer `definitely_unstarted` or clear the claim; replay can only observe durable state.
- The DB transaction serializes the journal claim with lifecycle updates to the Run row. PostgreSQL behavior still needs rerun with the configured test DSN.
- A standalone pre-implementation behavioral RED was not recorded because the service and S1 seams were already uncommitted at handoff; focused behavior tests and the race test pass on the final checkpoint.
