# Craft #107 T01 RunView R5 engine Fix1 Task 1 report

## Scope

Closed the two R5 Engine Task 1 review findings in `internal/container/craft_runview_docker_engine.go` and `_test.go`. The adapter now rejects shared or unknown process and namespace modes. The disposable Docker proof now reads and attempts a write at the inspected container knowledge mount destination. No provider, H2, runtime assembly, DI, image publication, commit, or push changes were made.

## RED → GREEN evidence

- Against the reviewed host-only checks, `TestCraftRunViewDockerEngineRejectsNonPrivateNamespaces` failed as expected for `container:<id>` PID/IPC modes, unknown PID/IPC/UTS/userns/cgroupns modes, shareable IPC, and container cgroup namespace. The regression test was then rerun after restoring the fix and passed.
- The adapter now allows only Docker's default empty PID mode, empty or explicit private IPC mode, empty UTS/userns modes, and empty or explicit private cgroup namespace mode. Host, container-shared, special, and unknown variants fail closed before the provider can certify the container.
- The live test resolves the knowledge destination from `InspectContainer` mounts, confirms the challenge file is readable through that exact destination, attempts to append as root, requires the error text `Read-only file system`, confirms the host bytes remain unchanged, and then cleans up and reinspects the temporary container and network.

## Verification

All commands ran in the integration worktree at `a5e9195acd6500c085c85d60c852148e7bbbbf34` after the concurrent R4 package run completed.

- `go test ./internal/container -run '^TestCraftRunViewDockerEngineRejectsNonPrivateNamespaces$' -count=1 -v` — PASS with host/container/unknown variants and explicit-private allowlist cases.
- `go test ./internal/container -run '^TestCraftRunViewDockerEngine|^TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity)' -count=1` — PASS.
- `go test -race ./internal/container -run '^TestCraftRunViewDockerEngine|^TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity)' -count=1` — PASS.
- `DOCKER_HOST="$(docker context inspect -f '{{.Endpoints.docker.Host}}')" CRAFT_RUNVIEW_R5_DOCKER_TEST=1 go test ./internal/container -run '^TestCraftRunViewDockerEngineRealPinnedProviderIsolation$' -count=1 -v` — PASS on local Linux/arm64 Docker. Test logged image ID/repo digest `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11`; container ID `6257f12a4f0826c1d49b106c787444a82511ce034bd9e4ba3cb4ec6faf1a1b11`; network ID `e3c20c5944d586f3ac74fa9490a9c995da8abc0db04ed6d3a8e78cd372e15ce0`; OpenCode `1.18.4`; binary SHA256 `3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e`; config SHA256 `c211b994a7cdb88dac8b119e140b9eda29e9bd64e8ba59e63c5531484f0e7dde`. The destination read succeeded, append failed specifically with read-only-filesystem error, host content stayed `original`, and cleanup verified container and network absence.
- `gofmt -d internal/container/craft_runview_docker_engine.go internal/container/craft_runview_docker_engine_test.go` — no output. `git diff --no-index --check` against the exact preimage files emitted no whitespace diagnostics (exit 1 indicates expected differences). `git apply --stat` parsed the task-local patch. The linker emitted `ignoring duplicate libraries: '-lc++'`; tests passed.

## Remaining limits

The local proof does not supply the missing deployable registry digest, production assembly/DI, or production enablement. Those remain separate gates.

## Exact checkpoint

`2026-09-24-craft-107-runview-r5-engine-fix1-task1-checkpoint.json` records base HEAD, preimage and final file hashes, the task-local patch/report hashes, commands, and live Docker identities. `2026-09-24-craft-107-runview-r5-engine-fix1-task1-task-local.patch` is the delta from the reviewed R5 Task 1 checkpoint. No commit was created.
