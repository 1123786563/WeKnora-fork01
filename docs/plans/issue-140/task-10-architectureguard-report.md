# T10 Architectureguard Baseline Report

- Task: Evaluation plan Task 2, architectureguard current route baseline.
- Worktree base: `7c8a7735ecfd60f5abbbded972728a749cc04b7a`.
- Commit: `efed56a93c0e7db870ed7bd737fc019dea4c834a`.
- Files changed: `tools/architectureguard/discovery_test.go`, `docs/architecture/moves/README.md`.
- Change: current expected route count is 650 (581 literal, 69 apiKeyRoute, 0 handle); README retains 647 as the prior checkpoint and records three T10 Evaluation Career routes.

## Verification

- `go test -count=1 ./tools/architectureguard -run 'TestGuardCleanAtHead|TestDiscoverRealRepoRouteTotals' -v` — PASS.
- `go run ./tools/architectureguard --root .` — PASS; 581/69/0, total 650, Redis/Lite 23/23, hooks 58, modules 17, zero violations.
- `git diff --check` — PASS.
- `go test -count=1 ./...` — FAIL (exit 1). Most packages passed, including `tools/architectureguard`; `internal/modules/agentruntime/agent/opencode` timed out after 10m in `TestLiveLockedBinaryTwoConsecutiveRoundsWithLocalMock` while `startLockedServe` awaited its local HTTP server. Also `internal/modules/commercial/payment` failed `TestProvidersFromEnvRejectsPartialAlipay`: expected the missing variable name but received `WEKNORA_ALIPAY_PUBLIC_KEY_PATH` in the error. These failures are outside the two-file change.
