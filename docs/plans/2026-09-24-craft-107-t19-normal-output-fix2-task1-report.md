# T19 normal-output provider Fix2 Task1 report

Status: implementation checkpoint complete; ready for independent review.

## Scope and changes

Only `internal/modules/execution/sandbox/docker_normal_exec.go` and `_test.go` were changed for implementation. Captured preimage hashes are the exact Fix1 checkpoint (`40f1c21f53f39260f46515f33632aafa66aa2e74b7de96c69dfd063d285c0981` provider; `7cd44c1686f033b58f4d0eafa986e353ee67f42ee013dc71273723df9a67b027` tests).

- Replaced arbitrary typed callback functions with `DockerNormalExecOutputCallback` (`Deliver` + prompt `Seal`) and invoke `Deliver` outside the writer mutex. On drain/finalization the writer freezes first, then seals both sink and callback; late completion sees frozen state and does not mutate returned counters/errors.
- Rejected legacy `RemoteExecRequest.OnOutput` before `ExecCreate`, since an arbitrary function has no cancellation or seal contract and cannot safely be kept bounded while preventing late work. A focused test proves no Docker create or attach occurs for it.
- Added a blocking sealable callback test: cancellation returns after the configured drain bound, sealing occurs without waiting for callback function code, and releasing the callback later produces no post-return effect.
- Captured cancellation after full stream/input drain and after terminal inspection. `startCtx.Err()` is checked after drain, after exact-ID observation, and immediately before success return; cancellation returns partial plus its cause.
- Kept one `ExecAttach`, exact-ID inspection, sink `Append`/`Seal`, terminal-gated completeness, and typed callback panic recovery.

## TDD and verification

RED command:
`go test ./internal/modules/execution/sandbox -run '^TestDockerNormalExec(BlockedTypedCallbackDoesNotHoldDrainFreeze|BlockedLegacyCallbackDoesNotHoldDrainFreeze|CancellationAfterDrainBeforeInspectIsPartial)$' -count=1`

All three failed before the fix: both arbitrary function callbacks blocked provider return beyond the 5 second drain bound, and cancellation during delayed final inspect returned nil/complete rather than `context.Canceled`/partial. The legacy test was then converted into the explicit pre-create rejection assertion required by the final API contract.

GREEN:

- `go test ./internal/modules/execution/sandbox -run '^TestDockerNormalExec' -count=1` — PASS.
- `go test -race ./internal/modules/execution/sandbox -run '^TestDockerNormalExec' -count=1` — PASS.
- `go test ./internal/modules/execution/sandbox -run '^$' -count=1` — PASS package compile.
- `git diff --check -- internal/modules/execution/sandbox/docker_normal_exec.go internal/modules/execution/sandbox/docker_normal_exec_test.go` — PASS.

Exact outputs are saved in `2026-09-24-craft-107-t19-normal-output-fix2-task1-test-output.txt`.

No live Docker rerun was performed because this Fix2 changes callback lifecycle and cancellation classification, not ExecCreate/ExecAttach/stream framing or inspect transport. Fix1 already passed its final-hash pinned-image live probe.

## Checkpoint

HEAD is `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Final source/test hashes, preimage hashes and byte sizes are recorded in `2026-09-24-craft-107-t19-normal-output-fix2-task1-checkpoint.json`. Incremental patch from the exact Fix1 preimage is `2026-09-24-craft-107-t19-normal-output-fix2-task1-task-local.patch`; it contains only the two owned source/test files. No commit was created and concurrent worktree changes were left untouched.

## Remaining contract limits

Raw function callbacks cannot satisfy bounded return and no post-return work together, so the legacy `RemoteExecRequest.OnOutput` function is explicitly unsupported for this provider. Typed callback implementations must honor `Deliver` context and make `Seal` prompt and linearizable against side effects; an implementation violating that contract cannot be made safe by this adapter. The provider remains unrouted, and S2 durable claim responsibility is unchanged.
