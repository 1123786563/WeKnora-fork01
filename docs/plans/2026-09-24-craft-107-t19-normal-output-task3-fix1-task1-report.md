# Craft #107 T19 normal-output Task3 Fix1 Task1 report

Date: 2026-09-24. Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`. Scope: independent-review finding T3-1 only. The service source and focused service test are the only code files changed; no provider, store, coordinator, migration, route, or Docker files were changed for this fix.

## Change

`OutputComplete` now requires `startErr == nil` in addition to the existing positive-start, complete-transport, terminal-process, and sealed-output evidence. A provider returning complete-looking outcome evidence alongside a start error therefore retains the exact receipt and `StartEvidence`, but the result is incomplete/partial and the returned error remains `RemoteOperationUnknown`. The regression test checks replay does not issue another create or attach.

## TDD and verification

RED changed the fake provider response-loss case to return complete-looking transport, terminal, and start evidence together with `startErr`. Before the fix, the focused test failed at the `OutputComplete` false assertion (line 303). GREEN passed after making completeness error-aware.

Commands and results:

| Command | Result |
| --- | --- |
| `gofmt -w internal/application/service/craft_docker_normal_exec.go internal/application/service/craft_docker_normal_exec_test.go` | Passed |
| `go test ./internal/application/service -run '^TestCraftDockerNormalExecAttachResponseLossIsUnknownAndNeverResent$' -count=1 -v` (pre-fix) | Expected RED: failed because `OutputComplete` was true despite `startErr` |
| `go test ./internal/application/service -run '^TestCraftDockerNormalExecAttachResponseLossIsUnknownAndNeverResent$' -count=1 -v` (post-fix) | Passed |
| `TRPC_TEST_POSTGRES_DSN=<isolated parent-provided PG17 DSN> go test ./internal/application/service -run '^TestCraftDockerNormalExecProjectionOnSQLiteAndIsolatedPostgres$' -count=1 -v` | Passed SQLite and isolated PostgreSQL subtests |
| `TRPC_TEST_POSTGRES_DSN=<isolated parent-provided PG17 DSN> go test -race ./internal/application/service -run '^TestCraftDockerNormalExec' -count=1` | Passed, `ok github.com/Tencent/WeKnora/internal/application/service 58.152s` |

The isolated PostgreSQL credential/DSN is omitted. The focused regression preserves exact receipt identity, unknown classification, partial output, and one-send behavior across replay. No physical Docker rerun was needed because this finding is confined to fake-provider error classification; the Task3 physical proof is documented in the parent Task3 report.

## Review state

Independent Fix1 re-review is pending. The service remains unrouted. No commit was created.
