# Craft #107 R5 Effect Task 3b — Task 3 Report

## Status

Implementation and focused fake validation complete; independent Task 3 review remains pending. No commit was created.

## Scope and result

Task 3 sequences the admitted coordinator as read-only network observation and claim → network create → container observation and claim → container create → state observation and claim → start → probe observation and claim → one Docker probe send → OpenCode session observation/marker/claim → one session POST. Existing exact network/container/start evidence is claimed and recorded without another send. Unknown or pending claims fail closed and block every downstream effect.

Network and start response loss can be reconciled only by an exact independent observation; the durable receipt is built from that observation, not the lost response. Runtime-probe response loss remains `unknown` when read-only inspection cannot recover exact exec evidence, and replay is inspect-only with no second Exec. The coordinator rejects the legacy combined provider and leaves production/default-on wiring untouched.

Another worker had already advanced the two owned files after the parent-supplied Task 3 preimage was captured. That work was preserved. Before this worker's edits, an additional exact current preimage was captured at `2026-09-24-craft-107-runview-r5-effect-task3b-task3-current-preimage.tar.gz`; its owned-file SHA-256 values matched the live files byte-for-byte. This worker strengthened the fake response-loss/probe-replay coverage; no production runtime edit was needed beyond the inherited current implementation.

## RED / GREEN evidence

RED was run by overlaying the parent-supplied preimage runtime onto the current package and running the new response-loss test:

```text
go test -overlay=docs/plans/.task3-red-overlay.json ./internal/container -run '^TestCraftRunViewAdmittedResponseLossUsesExactObservationOrStaysUnknown$' -count=1
```

Result: FAIL as expected in both subtests because the preimage coordinator failed closed before the Task 2 split-provider sequence. The two temporary overlay files were removed after the run.

GREEN commands and exact results:

1. `go test ./internal/container -run '^TestCraftRunViewAdmittedResponseLossUsesExactObservationOrStaysUnknown$' -count=1 -v` — PASS (both subtests).
2. `go test ./internal/container -run '^TestCraftRunViewAdmittedProbeUnknownReplayNeverExecutesAgain$' -count=1 -v` — PASS; one Exec send, two read-only observations, no session send.
3. `go test ./internal/container -run '^TestCraftRunViewAdmitted.*' -count=1` — PASS.
4. `go test -race ./internal/container -run '^TestCraftRunViewAdmitted.*' -count=1` — PASS.
5. `gofmt -w` followed by `gofmt -d` over both owned Go files — clean.
6. `git diff --check -- internal/container/craft_runview_runtime.go internal/container/craft_runview_admitted_runtime_test.go` — clean.

The Go linker emitted the known macOS duplicate `-lc++` warning during test linking; every command exited successfully. No Docker daemon or live OpenCode endpoint was used.

## Checkpoint

- Base HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`
- Parent-supplied preimage: `2026-09-24-craft-107-runview-r5-effect-task3b-task3-preimage.tar.gz` (SHA-256 `49eccc40b68488278b30736ff3fe1a714f3ce7fde138e4a8862fc9d56e7099f0`; superseded by concurrent owned-file edits)
- Exact preimage before this worker's edits: `2026-09-24-craft-107-runview-r5-effect-task3b-task3-current-preimage.tar.gz` (SHA-256 `da4735a7b89a5261287c8ff9634de51335f8ab887261df2708a0f3393cde3418`)
- Postimage: `2026-09-24-craft-107-runview-r5-effect-task3b-task3-postimage.tar.gz` (SHA-256 `9084a7ceda9b9aaeef6f038ed07acfe5d1917cb7ccdb1bcc268c3a9ba6923143`)
- Full Task 3 patch from parent-supplied preimage: `2026-09-24-craft-107-runview-r5-effect-task3b-task3-task-local.patch` (SHA-256 `4d9e32893c747815a17c93a5e9d48a4c38c498e8d3e54b36c17d948524091f00`)
- Detailed hashes: `2026-09-24-craft-107-runview-r5-effect-task3b-task3-checkpoint.json`

## Constraints and residual risk

- No adapter, repository, migration, DI, production assembly, or default-on configuration files were edited.
- Runtime probe replay remains intentionally unresolved when Docker inspection cannot prove the exact prior exec receipt; this is fail-closed, not retryable.
- Independent reviewer validation has not yet run for Task 3.
