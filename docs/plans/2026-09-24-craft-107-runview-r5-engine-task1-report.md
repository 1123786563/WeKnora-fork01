# Craft #107 T01 RunView R5 engine Task 1 report

## Scope

Implemented the isolated Docker SDK adapter for `CraftRunViewContainerEngine` and tested it against the provider seam. No central assembly, DI, production enablement, image publication, commit, or push was performed. The narrowly authorized provider and H2 fixture changes project only observed Docker image identity.

## RED → GREEN evidence

- Initial focused test compile was RED because the adapter factory and exact create request validator were not implemented.
- Added fake Docker API tests for private network creation and 404/error distinction, full inspected state projection, image-ID consistency, single-call create/start ambiguity, and runtime probe output/exit handling.
- The adapter uses bounded Docker API calls, only treats definitive not-found as absence, never retries ambiguous create/start, reports actual config/host/network/mount/image facts, and probes actual in-container version and file hashes.
- Provider validation now requires a nonempty `ImageID`. Adapter obtains it from `ContainerInspect.Image` and requires the matching `ImageInspect.ID` to be the same value. The provider does not synthesize identity from requested image fields.

## Verification

All commands ran in the integration worktree at base `a5e9195acd6500c085c85d60c852148e7bbbbf34`.

- `go test ./internal/container -run '^TestCraftRunViewDockerEngine|^TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity)' -count=1` — PASS.
- `go test -race ./internal/container -run '^TestCraftRunViewDockerEngine|^TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity)' -count=1` — PASS.
- `DOCKER_HOST="$(docker context inspect -f '{{.Endpoints.docker.Host}}')" CRAFT_RUNVIEW_R5_DOCKER_TEST=1 go test ./internal/container -run '^TestCraftRunViewDockerEngineRealPinnedProviderIsolation$' -count=1 -v` — PASS against local Docker Linux/arm64. Actual image ID and repo digest: `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11`; container and private network IDs were logged by the test; actual OpenCode version `1.18.4`; binary SHA256 `3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e`; config SHA256 `c211b994a7cdb88dac8b119e140b9eda29e9bd64e8ba59e63c5531484f0e7dde`; root write through the knowledge mount was denied; cleanup verified both container and network absent.
- `DOCKER_HOST="$(docker context inspect -f '{{.Endpoints.docker.Host}}')" CRAFT_RUNVIEW_LINUX_MOUNT_TEST=1 go test ./internal/container -run '^TestCraftRunViewProviderCreatedLinuxContainerKnowledgeMount$' -count=1 -v` — PASS; existing H2 Docker CLI fixture now projects the observed `.Image` field.
- `ld` printed `ignoring duplicate libraries: '-lc++'` during Go test linking; tests passed.

## Limitations

The repository's image lock still has a null registry digest, so the adapter constructor/config path remains fail-closed until deployable server-owned pins are supplied. The local test proves the installed image's repository digest, but does not publish or enable it. This task does not assemble the engine into production DI or enable RunView production use.

## Exact checkpoint

`2026-09-24-craft-107-runview-r5-engine-task1-checkpoint.json` contains base HEAD, task file SHA256 values, verification commands, and results. `2026-09-24-craft-107-runview-r5-engine-task1-task-local.patch` contains full additions for the two new adapter files and the authorized narrow ImageID/cleanup/logging edits to preexisting provider/H2 files. No commit was created.
