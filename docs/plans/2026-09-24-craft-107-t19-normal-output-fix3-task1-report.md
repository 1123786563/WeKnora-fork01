# T19 normal-output provider Fix3 Task1 report

Status: implementation checkpoint complete; ready for independent review.

## Scope and changes

Owned only `internal/modules/execution/sandbox/docker_normal_exec.go` and `docker_normal_exec_test.go`.

- Added a request wrapper that carries typed callback intent into `CreateAttachedExec` and rejects typed or legacy callbacks with `ErrDockerNormalExecCallbackUnsupported` before any Docker create/attach call.
- Removed direct callback execution and its seal/panic lifecycle from the transport. Accepted output goes only to the durable `Append/Seal` sink. Documentation says live projections must consume persisted output under their own cursor/cancellation contract.
- Added adversarial typed callback implementations with blocking `Seal` and ineffective `Seal`; both are rejected before create and the provider never invokes either `Deliver` or `Seal`. Retained pre-create legacy callback rejection.
- Preserved attached single-start transport, bounded writer and sink freeze, cancellation observation, output limits, demux and EOF/terminal-state behavior.

## TDD and verification

RED used the Fix2 lifecycle behavior before replacement. Command:
`go test ./internal/modules/execution/sandbox -run '^TestDockerNormalExecUnboundedTypedCallbackImplementationsAreUnsafe$' -count=1`
It failed as expected: blocking `Seal` kept execution beyond the drain bound, and an ineffective `Seal` allowed a callback side effect after provider return.

GREEN evidence and exact output are saved in `2026-09-24-craft-107-t19-normal-output-fix3-task1-test-output.txt`:

- Focused callback and transport regression tests — PASS.
- `go test ./internal/modules/execution/sandbox -count=1` — PASS.
- Focused `go test -race` — PASS.
- Package compile via `go test ./internal/modules/execution/sandbox -run '^$' -count=1` — PASS.
- `git diff --check` on the owned provider files — PASS.

No live Docker rerun: this checkpoint changes callback admission and lifecycle only, not the actual Docker ExecCreate/ExecAttach call or stream framing. The earlier Fix1 final-hash live Docker transport probe remains the applicable physical transport evidence.

## Checkpoint

HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`.

Preimage hashes are the exact Fix2 final checkpoint (`e23b284e562a7e36a5adc42398ac473695494221599409f8c91b5c200bf37db1` provider and `46e6606afb964afffdd1e83be4e1180b41c69acf9b165c0c5f5349538df47df2` tests). Final hashes and byte sizes are in `2026-09-24-craft-107-t19-normal-output-fix3-task1-checkpoint.json`. The first saved patch was a full `/dev/null` file snapshot and was corrected after independent review. The current task-local patch is an exact two-file Fix2→Fix3 unified delta. Its SHA-256 is `1ab26333587c376b4105d112988536350ee6b06074307aac848fa3fda7243b44` (28,577 bytes). To verify the base, I reconstructed the initial files by applying the initial normal-output, Fix1, and Fix2 patches into an empty temporary directory; hashes matched each prior checkpoint, including the Fix2 preimage hashes above. Applying the corrected Fix3 patch to a copy of those Fix2 files reproduced both Fix3 postimage hashes exactly. Commands and per-stage hashes are in the checkpoint JSON. Test output remains saved beside this report. Concurrent shared-worktree changes were left untouched. No commit was created.

## Remaining limits

Direct output callbacks are explicitly unsupported in this provider. A live projection feature needs a separate durable-store reader and cancellation/cursor contract. Production routing and its durable sink remain outside this Task1 scope.
