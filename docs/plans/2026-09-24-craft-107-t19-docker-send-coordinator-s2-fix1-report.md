# Craft #107 T19 Docker Send Coordinator S2 Fix 1 Report

## Result

Closed the S2-R1 cross-protocol adoption path with a durable nullable `protocol` discriminator on `craft_charge_start_journal`:

- Existing and legacy rows stay `NULL`; the paired additive migrations preserve these rows and constrain non-null values to `docker_coordinator`.
- Coordinator preparation writes the marker in the same transaction as the Task Budget hold and journal intent. Replay, `ResumeBound`, `Observe`, repository bind, and repository claim require the marker. An unmarked outstanding legacy intent is rejected and never adopted.
- Both legacy `BeginBinding` and `StartBinding` continue to prepare legacy rows with `protocol IS NULL`. Their shared prepare path conflicts on any existing key. Legacy outcome resolution now conditionally updates only an unresolved, unmarked row with no Docker receipt or send claim. A stale legacy handle therefore cannot resolve a coordinator-owned or claimed row.
- The Run revision/status lock and the receipt/claim CAS remain in the same repository transaction.

The ownership marker is never retrofitted onto an existing legacy intent. Under mixed-version overlap, an old writer's new row remains unmarked and the coordinator rejects it; if the coordinator inserts first, the legacy insert conflicts on the existing activity key and cannot return a callback attempt. This protocol does not enable Docker transport. S2-R3 remains open for the S3 adapter's physical callback, cancellation, and ambiguous response tests.

## RED evidence

Before the protocol guard was added, this command failed as expected because the coordinator adopted a legacy intent:

`go test ./internal/application/service -run '^TestCraftDockerSendCoordinatorRejectsLegacyIntentAdoption$' -count=1`

The new regression now holds a legacy `StartBinding` callback outstanding while coordinator preparation is attempted, verifies coordinator rejection, then proves a coordinator-owned row rejects legacy preparation and stale legacy resolution both before and after receipt binding/claim. The legacy callback count stays zero for coordinator-owned activity.

## Changed files

- `internal/application/service/craft_budget.go` — nullable protocol field, explicit legacy/Docker prepare paths, and legacy resolve CAS guard.
- `internal/application/service/craft_docker_send_coordinator.go` — require and preserve Docker protocol ownership on prepare replay, bound recovery, and observation.
- `internal/application/service/craft_docker_send_coordinator_test.go` — outstanding legacy callback interleaving, cross-protocol prepare/resolve rejection, process reconstruction, and disposable PostgreSQL coordinator coverage.
- `internal/application/repository/craft_docker_send_claim.go` — receipt binding and claim CAS require `protocol='docker_coordinator'`.
- `internal/application/repository/craft_docker_send_claim_test.go` — protocol fixtures, current-Run race coverage, and legacy-row migration preservation.
- `migrations/versioned/000197_craft_docker_send_protocol.{up,down}.sql`.
- `migrations/sqlite/000118_craft_docker_send_protocol.{up,down}.sql`.

The migration SQL is ignored by the repository's global migrations ignore rule and is present in the final uncommitted diff using intent-to-add entries. No other migrations, T01/H2 files, provider adapter, migration runner, or production wiring were changed. No commit was created.

## Verification evidence

SQLite 3.54.0 and disposable PostgreSQL 17.11 were exercised.

| Command | Result |
| --- | --- |
| `go test ./internal/application/service -run '^TestCraftDockerSendCoordinator|TestCraftBudgetStart|TestCraftChargeStart|TestCraftBudget.*Journal|TestCraftT19Journey' -count=1` | PASS |
| `go test -race ./internal/application/service -run 'TestCraftDockerSendCoordinator(ClaimIsOneOwnerAndReplayCannotResend|RejectsLegacyIntentAdoption)$' -count=1` | PASS |
| `go test ./internal/application/repository -run '^TestCraftDockerSend(Claim|Protocol)' -count=1` | PASS |
| `go test -race ./internal/application/repository -run '^TestCraftDockerSendClaim(ConcurrentOwnerRace|SerializesConcurrentRunPause)$' -count=1` | PASS |
| `TRPC_TEST_POSTGRES_DSN=<disposable PostgreSQL 17 DSN with app.skip_embedding=true> go test ./internal/application/repository -run '^TestCraftDockerSend(Claim|Protocol)' -count=1` | PASS, all SQLite and PostgreSQL subtests ran; includes concurrent claim and a status transition that holds the Run row lock before claim, plus migration and stale epoch/status checks |
| `TRPC_TEST_POSTGRES_DSN=<disposable PostgreSQL 17 DSN with app.skip_embedding=true> go test ./internal/application/service -run '^TestCraftDockerSendCoordinatorPostgresProtocolGuards$' -count=1 -v` | PASS; log recorded `server_version=17.11` |
| `gofmt -d` on all five Go files | No output |
| `git diff --check` | No output |

The full migration stream's optional embedding migration requires pgvector/pg_search extensions. The disposable test DSN used startup option `app.skip_embedding=true`, the migration's supported test/deployment switch; all Craft, journal, and protocol migrations still ran. The container was disposable (`postgres:17-alpine`), had no mounted data, and was removed after tests.

## Checkpoint

- Base HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit was created.
- Exact postimage snapshots and file SHA-256 values: `docs/plans/2026-09-24-craft-107-t19-docker-send-coordinator-s2-fix1-checkpoint.json` and its `snapshot/` directory.
- Task-local patch is `docs/plans/2026-09-24-craft-107-t19-docker-send-coordinator-s2-fix1-task-local.patch`; it is generated against the exact S2 Task 1 snapshot hashes plus four new migration files.
- The five Go preimages are retained in `2026-09-24-craft-107-t19-docker-send-coordinator-s2-task1-checkpoint/snapshot/` and their hashes are recorded in the fix1 manifest.

## Remaining scope and risks

- S2-R1 is addressed. Existing unmarked legacy rows are not adoptable and remain on their prior legacy protocol path.
- S2-R2 is addressed with actual PostgreSQL 17.11 repository and service evidence, including lock contention, cross-process claim, current status/revision, and migration tests.
- S2-R3 is deliberately not claimed: these tests exercise durable ownership and repository permission only. They do not call a physical Docker `ExecStart`; that proof belongs to S3.
- Deployment must apply the additive migration before using the updated coordinator service. Existing rows remain `NULL`; no coordinator feature is wired or enabled in this task.
