# T01 RunView R2 executor binding — independent scoped review

## Verdict

- **Scoped Spec compliance: PASS.** The executor no longer selects `Workspace.OpenCodeSessionID`. `Execute`, `Observe`, and `Abort` each resolve the Run key from the Task scope and fence, reject a mismatched or incomplete binding before runtime I/O, and use only that call's resolved client, session, and immutable directory. The old `NewExecutor` constructor fails closed.
- **Scoped code quality: PASS.** Prompt ID persistence precedes runtime submission; failed preflight observation returns unknown without a prompt; accepted retries inspect the persisted prompt ID rather than submit it again. The existing normalizer discards foreign session and foreign message events. The shared `Executor` stores no Run-specific client. No critical, high, medium, or low finding was established in this task-local checkpoint.
- **Production T01/RunView acceptance: NOT VERIFIED.** R5 must implement and inject a persisted RunView resolver and assemble the container runtime. This review does not establish Linux image, OS isolation, per-Run process isolation, or live Run A→B exclusion.

## Evidence

1. The checkpoint HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`. The four owned-file SHA-256 values and the patch/report SHA-256 values match `2026-09-24-craft-107-runview-r2-executor-binding-checkpoint.json`. `git apply --reverse --check` accepted the checkpoint patch against the current owned-file content; `git diff --check` found no whitespace error.
2. `run_binding.go:29-63` derives the exact `(tenant, owner, session, run)` key, checks the fence tenant/owner/epoch, compares the returned key, and rejects nil/unbound clients, invalid session IDs, and directory mismatches. The resolver interface requires a persisted RunView reload; verifying that implementation remains R5 work.
3. `executor.go:101-188,310-366` uses a local binding on each operation. `boundWorkspace` checks Task workspace ownership but never reads its old OpenCode session. `Execute` persists the prompt ID with `PrepareTask` before preflight and prompt; an unreadable preflight returns `ErrUnknown`. `NewExecutor` has no resolver and returns `ErrUnsupported` before any runtime call.
4. `TestRunBoundExecutorSelectsSeparateRunForEveryOperation` exercises two Runs under one Task workspace through Execute/Observe/Abort and records distinct session IDs and directories. `TestRunBoundExecutorFailsClosedWithoutValidBinding` covers the old constructor, mismatched Run, and unbound client. Existing `TestSubStateTracksOnlyTheBoundSessionAndTurn` verifies foreign event filtering; the unknown-acceptance test verifies no second POST after an uncertain first acceptance.
5. Independent checks: focused `go test` on RunBound/unknown acceptance/foreign-event tests passed; the same selection under `-race` passed; full `go test ./internal/modules/agentruntime/agent/opencode -count=1 -timeout=75s -v` passed in 25.779 seconds, with the live two-turn test skipped because `CRAFT_LIVE` was unset. An earlier unbounded full run stalled for over three minutes and was interrupted for diagnosis; the bounded rerun passed, so that first attempt supplies no pass evidence.

## Review boundary

This is a review of the exact R2 executor patch, not an approval of the future resolver implementation or the production container/provider assembly. The resolver's claim that its client/session came from the persisted RunView is a required interface obligation that must be verified in R5.
