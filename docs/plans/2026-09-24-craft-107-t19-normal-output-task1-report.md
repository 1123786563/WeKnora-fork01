# T19 normal-output Docker provider Task1 report

Status: implementation checkpoint complete; ready for independent review.

## Scope and implementation

Owned only `internal/modules/execution/sandbox/docker_normal_exec.go` and `_test.go`. Added an inert attached-capable ExecCreate receipt, exactly one hijacked `ExecAttach` as the only provider start operation, stdin hash/count validation and one `CloseWrite`, bounded non-TTY frame parsing and output forwarding to a sink before callbacks, typed complete/partial/unavailable transport and process observation, exact-ID preflight/postflight inspection, positive start evidence, and process-local rejection of a second attach attempt. The provider interface intentionally omits detached `ExecStart`.

Fake coverage checks create-time stream options, zero starts before the claim call, wrong IDs and altered receipts, stdin half-close, split stdout/stderr frames including binary bytes, no retry after lost attach response, output quota, sink/callback failures, malformed frames, exact-ID observation, false-zero unknown state, input overflow and required sink.

## TDD and verification

RED: the initial focused test compile failed because the new provider types/methods were not yet implemented (`undefined` provider API); implementation followed the test expectations.

GREEN commands/results:

- `go test ./internal/modules/execution/sandbox -run '^TestDockerNormalExec' -count=1` — PASS.
- `go test -race ./internal/modules/execution/sandbox -run '^TestDockerNormalExec' -count=1` — PASS.
- `DOCKER_HOST=unix:///var/run/docker.sock CRAFT_T19_NORMAL_OUTPUT_DOCKER_TEST=1 go test -v ./internal/modules/execution/sandbox -run '^TestDockerNormalExecRealPinnedNoEgressTransport$' -count=1` — PASS.

The live probe used the lock-recorded image `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11` and verified Linux/arm64 identity. It created no-egress disposable container `91998ef06e3de8b534dd025da3c51da9062cdb958f10514f2482c4bdeee7381e` and exact exec `e2351c45c4055caa4cb79dbd6e578212f89092c7908f8fbd91d6e2ef49bcf6df`. The command consumed stdin through EOF, returned separated `stdout` and `stderr` marker lines, exact-ID inspect reported exit 0 after positive start evidence, and a file copied from the container held exactly one marker line. Cleanup removed the container and a subsequent inspect confirmed it absent.

## Checkpoint

Base HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Both owned files were absent at that base. Exact final SHA-256 hashes, byte sizes and commands are in `2026-09-24-craft-107-t19-normal-output-task1-checkpoint.json`. Task-local patch is `2026-09-24-craft-107-t19-normal-output-task1-task-local.patch`; it contains only the two owned files.

Other concurrent worktree changes were present and were left untouched. No commit was created.

## Remaining risks / limits

- The provider-local second-start guard is process-local; a durable S2 claim remains the caller's responsibility. This provider does not advertise `RemoteExecInitiator` and is not wired into production.
- After ambiguous attach failure, output is unavailable and the provider never retries. The coordinator must preserve the unknown/partial result and budget hold/fence.
- Duration is not fabricated from inspect or local poll time; Docker inspect provides no authoritative process duration.
