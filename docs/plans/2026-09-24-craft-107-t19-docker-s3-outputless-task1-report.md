# Craft #107 T19 S3 Docker outputless initiation — Task 1 report

Status: DONE_WITH_CONCERNS (scoped implementation and checks complete; TDD red evidence was not captured before the initial implementation draft).

## Scope delivered

- Added a separate `DockerOutputlessExecRequest` requiring explicit discard, and rejected missing opt-in, stdin, output callbacks and invalid command/target before durable hold or Docker create.
- Added inert Docker `ExecCreate`, bounded one-call detached `ExecStart`, and same-ID `ExecInspect` observation. The path never calls `ExecAttach`, never restarts a stopped container, and reports stdout/stderr as unavailable.
- Added `CraftDockerRestrictedExec` composition over the S2 durable hold, exact receipt binding, exclusive claim and single-use permission. A lost start response is returned as unknown; a replay cannot receive a second start permission. Observation requires claimed durable state and preserves false/zero, mismatched identity and inspect failures as unknown.
- Left central container assembly, Run admission, ordinary command routing, generic `RemoteExecInitiator`, and legacy Docker execution unchanged.

## Changed files

- `internal/modules/execution/sandbox/docker_restricted_exec.go`
- `internal/modules/execution/sandbox/docker_restricted_exec_test.go`
- `internal/application/service/craft_docker_restricted_exec.go`
- `internal/application/service/craft_docker_restricted_exec_test.go`
- `docs/plans/2026-09-24-craft-107-t19-docker-s3-outputless-task1-task-local.patch`
- `docs/plans/2026-09-24-craft-107-t19-docker-s3-outputless-task1-checkpoint.json`
- This report.

## Verification evidence

- `go test ./internal/modules/execution/sandbox -run '^TestDockerRestricted' -count=1` — PASS.
- `go test ./internal/application/service -run '^Test(CraftDockerRestricted|CraftDockerSendCoordinator(PreparesHeldIntentThenBindsExactReceipt|ClaimIsOneOwnerAndReplayCannotResend))' -count=1` — PASS.
- `go test -race ./internal/modules/execution/sandbox -run '^TestDockerRestricted' -count=1` — PASS.
- `go test -race ./internal/application/service -run '^Test(CraftDockerRestricted|CraftDockerSendCoordinator(PreparesHeldIntentThenBindsExactReceipt|ClaimIsOneOwnerAndReplayCannotResend))' -count=1` — PASS.
- `DOCKER_HOST=<active-context> WEKNORA_DOCKER_RESTRICTED_INTEGRATION=1 go test ./internal/modules/execution/sandbox -run '^TestDockerRestrictedExecRealDockerMarkerOnce$' -count=1 -v` — PASS against Docker 29.4.0 client/server, Moby client v0.5.1 and `wechatopenai/weknora-sandbox:main`. The test started one outputless marker command detached, observed the same exec ID running and terminal, copied the marker file without another exec, verified exactly `marker\n`, and removed the isolated network-none container.
- `git diff --check -- <four owned source/test files>` — PASS.

The exact four-file task checkpoint and SHA-256 values are in `2026-09-24-craft-107-t19-docker-s3-outputless-task1-checkpoint.json`; patch SHA-256 is `28ddc39b152f8e13c917c44af229a09e79e3b8da575611af5be4638e6866d760`. Base HEAD was `a5e9195acd6500c085c85d60c852148e7bbbbf34`. No commit was created. Other pre-existing integration-worktree modifications were not included.

## Assumptions and remaining limits

- The output-discard request type is the explicit caller opt-in for this restricted capability; stdout and stderr are never represented as empty strings.
- Running evidence is process-local. After process restart, an inspect reporting false/zero remains unknown because S2 does not persist a positive running observation. Durable reconciliation and release of the hold/fence remain outside this task.
- The real probe exercises the physical Docker transport directly; coordinator claim exclusivity and ambiguous-response no-retry are verified with the coordinator composition fake. No production runtime wiring was added, per the assigned scope.
- TDD RED was not recorded before the first implementation draft. New targeted behavior tests, race tests, and the real Docker probe pass on the final checkpoint.
