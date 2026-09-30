# T19 gateway body and initiation deadline correction report

**Status:** implementation checkpoint ready for independent review; no commit created.

## Scope and design

Implemented the approved two-phase seam in `docs/plans/2026-09-23-craft-107-t19-gateway-two-phase-fix-plan.md`.

The service now prepares the journal intent and G4 hold transactionally and returns an opaque attempt with a bounded initiation context. The HTTP transport runs outside SQL. A state-gated cancellation callback cancels blocked `Do` only until response headers return; the existing request/forward context continues to bound the response body. The handler reads the entire bounded body and closes it before resolving `started`. Send/body/close/oversize/empty-body failures resolve `unknown`; a failed resolver is surfaced as gateway failure while durable intent and its hold remain. Pre-send failure can resolve `definitely_unstarted`. Resolution is one-shot CAS from `intent`, so an `unknown` row cannot later become `started`. Existing `StartBinding` remains available with its prior callback contract.

No repository seam or migration was required: the existing journal state transition supports the conditional update. Separate activity-ID provenance finding remains out of scope and unresolved.

## RED → GREEN evidence

Before handler correction, the new behavior tests failed in the intended ways: post-send body read failure had already resolved as `started`; a real `httptest` server blocking response headers returned only at the full response timeout (~1s), exceeding the configured 60ms initiation timeout. A dedicated empty-body case also initially returned HTTP 200. After the implementation each regression case passed.

Final commands and results on the checkpoint source:

- `go test ./internal/handler -run 'TestCraftGateway|TestCraftModelActivityKey' -count=1 -timeout=30s` — PASS (`1.371s`). Includes real delayed-body and blocked-Do `httptest` cases, request cancellation, body read/close/nil/empty/oversize errors, and resolver failure.
- `go test ./internal/application/service -run '^TestCraftChargeStart' -count=1 -timeout=30s` — PASS (`10.736s`). Includes committed intent/hold visibility, durable cancel precedence through resolution/recovery, failed result persistence preserving intent/hold, and non-replayable unknown.
- `go test -race ./internal/handler -run 'TestCraftGateway|TestCraftModelActivityKey' -count=1 -timeout=60s` — PASS (`4.062s`).
- `go test -race ./internal/application/service -run '^TestCraftChargeStart' -count=1 -timeout=60s` — PASS (`9.273s`).
- `go test ./internal/application/repository -run 'CraftCharge|CancelRun|DeleteSession|Cancel' -count=1 -timeout=60s` — PASS (`25.010s`); focused durable journal/cancellation persistence coverage.
- `git diff --check -- internal/handler/craft_model_gateway.go internal/handler/craft_model_gateway_test.go internal/application/service/craft_budget.go internal/application/service/craft_budget_start_test.go` — PASS.
- `git apply --check --reverse docs/plans/2026-09-23-craft-107-t19-gateway-two-phase-checkpoint.patch` — PASS; checkpoint patch reverses cleanly against this worktree.

## Snapshot and exact checkpoint

Integration worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`.
Starting integration HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged at checkpoint). The implementation plan's original project BASE is `4bcad69baf033a1310b4dce1372c8153e66adc81`.

The owned files were already modified/untracked before this task. Pre-task tracked staged patch was empty (SHA-256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`); pre-task unstaged patch SHA-256 `afd6f506756439a0a8663b3d755184597fdb578bd4f64b0cf640cfb1e4a39e85`; the pre-existing untracked service test was copied verbatim to `2026-09-23-craft-107-t19-gateway-two-phase-task-baseline-craft-budget-start-test.go` (SHA-256 `02faea72b26b066e959f8b40672706d42d7c30f7780f636fb887543c0f12b67a`). Pre-task owned-file hashes are recorded in the checkpoint JSON.

The exact combined owned-file diff from HEAD, including the pre-existing dirty state and this task's additions, is `2026-09-23-craft-107-t19-gateway-two-phase-checkpoint.patch`; it includes the untracked service test. Checkpoint and final owned-file hashes are recorded in `2026-09-23-craft-107-t19-gateway-two-phase-checkpoint.json`. This format preserves the shared baseline rather than misattributing earlier T19 edits to this task.

No unrelated source paths were edited by this task. The worktree has other concurrent modifications that remain untouched.
