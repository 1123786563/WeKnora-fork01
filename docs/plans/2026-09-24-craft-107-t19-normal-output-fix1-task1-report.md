# T19 normal-output provider Fix1 Task1 report

Status: Fix1 implementation checkpoint complete; ready for independent review.

## Scope and changes

Owned only `internal/modules/execution/sandbox/docker_normal_exec.go` and `_test.go`. The provider now keeps clean stream EOF provisional until exact-ID inspection shows a terminal process state; a still-running process leaves transport partial/unknown. Writer counters/errors are read from a mutex-protected frozen snapshot. The output sink is now a typed `Append`/`Seal` contract: after sealing, implementations must prevent in-flight or future appends from mutating durable output, and append must honor its cancellable context before commit. The writer freezes before sealing, and late append completion cannot update counts or invoke callbacks. Typed callback panics become `ErrDockerNormalExecCallbackPanic` and a partial outcome.

## TDD evidence

The three new RED tests reproduced the review findings before the fix:

- EOF while inspect reported Running returned nil error and `TransportComplete` (test failed).
- Cancellation while a sink was blocked returned after the drain bound; releasing the sink then caused a late append (test failed).
- A typed callback panic escaped the output goroutine and terminated its subprocess (test failed).

After implementation, the same focused suite passed.

## Verification

- `go test ./internal/modules/execution/sandbox -run '^TestDockerNormalExec' -count=1` — PASS.
- `go test -race ./internal/modules/execution/sandbox -run '^TestDockerNormalExec' -count=1` — PASS.
- `DOCKER_HOST=unix:///var/run/docker.sock CRAFT_T19_NORMAL_OUTPUT_DOCKER_TEST=1 go test -v ./internal/modules/execution/sandbox -run '^TestDockerNormalExecRealPinnedNoEgressTransport$' -count=1` — PASS at the final provider/test bytes.

Live Docker evidence: pinned image `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11` (Linux/arm64); disposable no-egress container `018e0bd8d6ad30d13646b30475a823e258dc9f5155ad1a0ea767a0ec07c8d0fd`; exact exec `3acaf25008ae6dc4f6d9d4e330b8ff6ee187a4683a58f5cad43af0f0fd8b4859`. The probe read stdin through EOF, separated stdout/stderr, inspected terminal exit 0 after positive start evidence, observed exactly one marker line, removed the container, and confirmed a subsequent inspect found it absent.

## Checkpoint

Worktree HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Preimage hashes are bound to the prior Task1 checkpoint and repeated in `2026-09-24-craft-107-t19-normal-output-fix1-task1-checkpoint.json`; final hashes/byte counts are there as well. The task-local patch is a delta from those exact preimage files and contains only the two owned files. No commit was created.

## Remaining constraints

The sink contract is semantic: implementations must make `Seal` prompt and ensure no durable append can occur after it returns. The provider cannot make a nonconforming sink safe. Output callbacks are invoked synchronously and must return promptly; panic is converted into a typed partial error. The provider remains unrouted and the durable S2 claim is still the caller responsibility.
