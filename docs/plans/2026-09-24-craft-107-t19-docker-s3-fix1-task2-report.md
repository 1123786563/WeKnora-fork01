# Craft #107 T19 S3 Fix1 — Task 2 report

**Base HEAD:** `a5e9195acd6500c085c85d60c852148e7bbbbf34`.  
**Owned change:** `internal/application/service/craft_docker_restricted_exec_test.go` only. No production code changed.

## Coordinator-level coverage

- A stale Run revision injected after inert create makes exact receipt Bind fail; no physical start callback occurs and the intent/hold remains unbound and unclaimed.
- A reconstructed budget service/coordinator resumes a receipt persisted before claim; ResumeBound starts the exact persisted container/exec ID and does not create another exec.
- A reconstructed service/coordinator after durable Claim cannot ResumeBound into another physical start; the exact receipt, claim and unresolved hold remain.
- A concurrent Start and ResumeBound race is held while the first provider callback runs. The callback checks the durable coordinator projection contains its exact claimed receipt before proceeding, records its provider-side effect, then returns a simulated lost response. Concurrent Start and ResumeBound replays receive no second physical callback. The final state remains `intent` with the exact receipt, `send_claimed_at` and dispatched reservation.

## Disposable real-Docker proof

Command:

```text
WEKNORA_DOCKER_COORDINATOR_INTEGRATION=1 go test ./internal/application/service -run '^TestCraftDockerRestrictedCoordinatorRealDockerResponseLossMarkerOnce$' -count=1 -v
```

Result:

```text
=== RUN   TestCraftDockerRestrictedCoordinatorRealDockerResponseLossMarkerOnce
Docker host=unix:///var/run/docker.sock client-api=1.54 server=29.4.0 api=1.54 image=wechatopenai/weknora-sandbox:main
container=f22cc418f8ad22b8d1209814ab41ceb540669b110399e9f692d2a4914bbbc360 exec=b78ee33786569db5fe25390191eb90d14321fca2e92a733e8d775aa729045c79 claimed=true claim_verified_at_callback=true callback_count=1 side_effect_count=1 marker="marker\n" container_log_bytes=0
cleanup container=f22cc418f8ad22b8d1209814ab41ceb540669b110399e9f692d2a4914bbbc360 force_remove=true error=<nil>
--- PASS: TestCraftDockerRestrictedCoordinatorRealDockerResponseLossMarkerOnce (6.35s)
PASS
ok   github.com/Tencent/WeKnora/internal/application/service 7.306s
```

The test creates an isolated `network=none` container and starts a delayed marker command through service `Start`: Prepare → inert Create → exact Bind → Claim → provider callback → detached Docker ExecStart. At the provider callback, the test queries the coordinator and verifies that `send_claimed_at` and the matching persisted container/exec IDs are already committed. The wrapper then simulates response loss after real ExecStart returns successfully. Start returns unknown; a ResumeBound replay is rejected; the callback count remains one. Wait inspects the same exact receipt to terminal success. Archive copy (no extra exec) reads exactly `marker\n`, proving one marker write. Container logs were empty (zero bytes); the test logs the cleanup response, and a follow-up `docker inspect <container-id>` returned `no such object`, confirming cleanup.

The exact Moby client module pinned by the repository is `github.com/moby/moby/client v0.5.1`; the integration run negotiated API 1.54 against Docker server 29.4.0. Local image digest at setup was `sha256:69c14626ee0f92ef01756939ee600b763e5caade8b176607fa0e84e28eb917e3`.

## Verification

```text
$ go test ./internal/application/service -run '^TestCraftDockerRestrictedCoordinator' -count=1
ok   github.com/Tencent/WeKnora/internal/application/service 2.596s
$ go test -race ./internal/application/service -run '^TestCraftDockerRestrictedCoordinator' -count=1
ok   github.com/Tencent/WeKnora/internal/application/service 3.912s
$ go test ./internal/application/service -run '^TestCraftDockerRestricted' -count=1
ok   github.com/Tencent/WeKnora/internal/application/service 8.001s
$ go test ./internal/application/service -run '^$' -count=1
ok   github.com/Tencent/WeKnora/internal/application/service 1.880s [no tests to run]
$ gofmt -d internal/application/service/craft_docker_restricted_exec_test.go
(no output)
$ git diff --check
(no output; exit 0)
$ docker inspect f22cc418f8ad22b8d1209814ab41ceb540669b110399e9f692d2a4914bbbc360 --format '{{.Id}}'
Error: no such object: f22cc418f8ad22b8d1209814ab41ceb540669b110399e9f692d2a4914bbbc360
```

The real-Docker command above passed after adding the callback-time durable claim assertion. The test-only task had no production change; test-harness compile/type assertion mistakes were corrected before the passing focused and race runs. No production defect requiring replanning was found.

## Checkpoint and remaining gates

Exact preimage hashes, task-local patch, postimage archive and final hashes are recorded in `2026-09-24-craft-107-t19-docker-s3-fix1-task2-checkpoint.json`; preimage: `2026-09-24-craft-107-t19-docker-s3-fix1-task2-pre.json`. No commit or production route change was made. S3-R1 authoritative command duration remains open; this task does not claim full S3 or T19 acceptance.
