# Craft #107 T19 normal-output Task 3 report

Date: 2026-09-24. Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`. Implementation scope: new normal Docker exec application service and focused service tests only. No production routing, provider implementation, repository implementation, migration, or output-store transaction code was changed in this task.

## Implementation

- Added `CraftDockerNormalExecService` to compose the typed normal coordinator, immutable staged input, Docker normal provider, one durable output sink, and cursor reader.
- The service creates an inert exec only after normal preparation/staging, binds and claims the full receipt, opens output storage before consuming the process-local permission, then calls the provider's one-shot attached-start method once.
- A receipt-bound preclaim replay resumes only the persisted receipt. A claimed replay verifies the original staged request and container/exec IDs, observes the provider and reads committed output without opening a writer or attaching again.
- The staged `StdinEnabled` flag is passed separately from stdin bytes for both create and start, preserving enabled-empty stdin identity.
- The service enforces each staged output limit at the durable sink and marks quota-limited, unsealed, unavailable, nonterminal, or transport-incomplete results partial. It returns the exact normal receipt, process state, transport state, output snapshot/completeness and committed cursor.
- The service is not registered in ordinary execution routing.

## TDD and verification

RED was observed before implementation: `go test ./internal/application/service -run '^TestCraftDockerNormalExec' -count=1` failed to compile because `NewCraftDockerNormalExecService` did not exist yet. After implementation and the explicit provider stdin-enabled API update, the focused test command passed.

Commands run and evidence:

| Command | Result |
| --- | --- |
| `gofmt -w internal/application/service/craft_docker_normal_exec.go internal/application/service/craft_docker_normal_exec_test.go` | Passed |
| `go test ./internal/application/service -run '^TestCraftDockerNormalExec' -count=1` | Passed; focused SQLite fake-path tests, including complete output, cursor reconnect, bound receipt resume, response-loss replay, input mismatch, after-claim cancellation, sink-open/seal failure, output quota and enabled-empty stdin. Final rerun after the physical test host fallback change passed. |
| `go test -race ./internal/application/service -run '^TestCraftDockerNormalExec' -count=1` | Passed earlier on the focused SQLite paths. |
| `TRPC_TEST_POSTGRES_DSN=<parent-provided isolated PG17 DSN> go test ./internal/application/service -run '^TestCraftDockerNormalExecProjectionOnSQLiteAndIsolatedPostgres$' -count=1 -v` | Passed on the SQLite subtest and isolated PostgreSQL schema. The test output was `PASS` for both subtests. The credential is intentionally omitted here. |
| `TRPC_TEST_POSTGRES_DSN=<parent-provided isolated PG17 DSN> go test -race ./internal/application/service -run '^TestCraftDockerNormalExec' -count=1` | Passed on the final focused service test set, including the SQLite and PostgreSQL projection case (`ok .../internal/application/service 32.884s`). The credential is intentionally omitted here. |
| `CRAFT_T19_NORMAL_OUTPUT_DOCKER_TEST=1 go test ./internal/application/service -run '^TestCraftDockerNormalExecDisposableNoEgressMarkerOnce$' -count=1 -v` | Passed against the pinned local image. Container was inspected with network mode `none`; one service exec returned cursor 1, claimed replay made one provider observation and no second create/start, copied marker content was exactly `x`, and cleanup removed then verified the container absent. The environment used the default Docker local socket and did not pull an image. |

Raw focused output is in `2026-09-24-craft-107-t19-normal-output-task3-test-output.txt`; raw physical Docker output, including the initial corrected assertion failure and cleanup evidence, is in `2026-09-24-craft-107-t19-normal-output-task3-docker-output.txt`. The physical assertion failure was only a named `container.NetworkMode` versus string comparison; it was corrected, then the same bounded test passed with cleanup verified. Provider empty-stdin API review has scoped PASS; its reviewer recorded one low-risk same-length substitution test gap outside this service task.

## Risks and boundaries

- A claim remains unresolved and is returned as `RemoteOperationUnknown` after any ambiguous postclaim failure. No path settles the commercial hold or promotes a build.
- Process positive-start evidence used during same-process claimed observation is volatile. After restart, a terminal false/zero inspect without retained positive-start evidence remains unknown by design.
- The durable output repository labels sealed output partial until the service joins it with positive start, terminal and complete transport evidence. Only that fully joined result clears the projected `Partial` flag.
- Independent Task3 review remains outstanding at this report revision.
