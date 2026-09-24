# Craft #107 R5 Effect Task3b — Task 2 Report

## Status

Implementation and focused validation complete; independent review remains pending. No commit was created.

## Scope and result

Task 2 adds the admitted split provider boundary and keeps legacy combined provider APIs available for existing callers. Read-only operations inspect the requested Docker resource and generation filesystem evidence; runtime probe observation explicitly returns unresolved because Docker cannot recover previously measured stdout hashes without another exec. Network/container creation, container start, runtime probe send, and OpenCode session creation remain separate operations intended to be called only after the coordinator has claimed the corresponding durable effect.

The Docker network create sends once, then inspects the exact Docker-returned network ID and verifies name, driver, internal flag, labels, scope, options, and attachment identity. Container create and start similarly reinspect the exact deterministic resource and verify their source-derived state. Runtime probe success includes the exact container ID and exec ID together with measured version/binary/config values; any ambiguity is returned unresolved and is not retried by this adapter. The request digest input contract remains at the Task1/Task2 interface: callers hash versioned canonical network or probe request data; the adapter does not canonicalize it.

## RED / GREEN evidence

- Engine focused tests were first run RED while the split methods were absent (recorded earlier in the shared Task2 execution). They were made GREEN by adding inspect-only network observation, one-send network create plus exact-ID receipt inspection, explicit unresolved read-only probe observation, and canceled-context send guards.
- Provider observation test was first run RED because the fake engine lacked the admitted split operations; fake operations/counters were added and the read-only/no-filesystem-change assertions passed.
- The canceled-send test was first run RED because a canceled context still reached the Docker API; context checks were added and the test passed.
- The live Docker integration test now exercises the split flow: network absent → one create → exact observe; container absent → one create → exact observe in created state → one start → running; probe observe unresolved without exec → one claimed probe send with exact receipt. It also retains the real read-only knowledge-mount write-denial challenge.

## Verification

Commands run from the integration worktree:

1. `go test ./internal/container -run '^(TestCraftRunViewDockerEngine.*|TestCraftRunViewAdmittedProvider.*)$' -count=1` — PASS.
2. `DOCKER_HOST=unix:///Users/wuyongjun/.orbstack/run/docker.sock CRAFT_RUNVIEW_R5_DOCKER_TEST=1 go test ./internal/container -run '^TestCraftRunViewDockerEngineRealPinnedProviderIsolation$' -count=1 -v` — PASS. The disposable generation used the pinned image repo digest; observed runtime was OpenCode 1.18.4 and the binary/config hashes matched the checked-in lock/config. The knowledge write challenge failed specifically because the mount was read-only. Test cleanup removed the container and network and asserted both were absent by inspection.
3. `go test -race ./internal/container -run '^(TestCraftRunViewDockerEngine.*|TestCraftRunViewAdmittedProvider.*)$' -count=1` — PASS.
4. `gofmt -d` over all four owned Go files — clean.
5. `git diff --check --` over all four owned Go files — clean.

The linker emitted a macOS duplicate `-lc++` library warning during Go test linking; tests exited successfully.

## Constraints and residuals

- This Task does not wire the coordinator or production DI. A durable claim is a caller precondition for each split send.
- `ObserveRuntimeProbe` intentionally cannot resolve a prior exec; a replay after uncertain probe send remains parked unless a higher layer has other exact durable evidence.
- The real Docker test uses a no-op OpenCode API factory and therefore does not assert live OpenCode session GET behavior. Provider-level focused tests and the OpenCode inventory implementation cover the read-only session seam; production GET failures are propagated as unresolved. No live session POST was sent.
- Independent Task2 review is pending.

## Checkpoint

- Base HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`
- Preimage archive: `2026-09-24-craft-107-runview-r5-effect-task3b-task2-preimage.tar.gz` (SHA-256 `9fca5a4cedb787571217a3d24d89049ebe79e20a3e53bac8b3d833e2aebf9238`)
- Postimage archive: `2026-09-24-craft-107-runview-r5-effect-task3b-task2-postimage.tar.gz` (SHA-256 `b34b32f9861effd14bd36006759ffbb9ecbec730862280b2e7fe2df827df2638`)
- Task-local patch: `2026-09-24-craft-107-runview-r5-effect-task3b-task2-task-local.patch` (SHA-256 `6d855cdf884b045bb5f035dee5eaea26359eac715509c246824cee3d2e28d33c`)
- Detailed file and artifact hashes: `2026-09-24-craft-107-runview-r5-effect-task3b-task2-checkpoint.json`
