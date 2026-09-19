# P0 Task 3 recovery evidence

Source revision: `d1d1857511f2c5d85a6213d07eba8c176541ea6e`.

## Commands and sanitized outcomes

| Command | Outcome | Evidence layer |
| --- | --- | --- |
| `GOWORK=off go test ./internal/agent/trpc -run 'TestCheckpointProbe|TestSQLiteSaverPendingWrites' -count=1 -v` | PASS. Four tests confirmed the disposable SQLite probe distinguishes an interrupted stream from completion, reopens its database, restores one pending write, and retains `call-probe-1`. | deterministic SQLite SDK probe |
| `GOWORK=off go test -race ./internal/agent/runtime -count=1 -v` | PASS. Runtime recovery-policy tests passed under the race detector. | unit/race policy evidence |
| `GOWORK=off go test -json ./internal/agent/recoverytest -count=1 > /tmp/weknora-native-recovery-p0.json` | PASS. Structured JSON recorded 18 passing named tests and no failures. The temporary log is intentionally outside the repository. | isolated SQLite process/fault harness |

The JSON pass set includes the SQLite crash matrix's ten barriers (`after_admission`, `after_plan_before_dispatch`, `after_result_before_checkpoint`, `after_finalize`, `after_side_effect_before_result`, `waiting_user`, `unknown_result_user_retry`, `idempotent_redelivery`, `oauth_park`, and `mcp_set_drift`), SQLite two-worker contention, and the five admission-gate checks.

## Structured skip parsing

`jq` selected three `Action=skip` records and zero `Action=fail` records:

| Skipped test | Environment variable / prerequisite | Status |
| --- | --- | --- |
| `TestCrashMatrixPostgreSQL` | `TRPC_RECOVERY_PG_DSN` unset; test creates a disposable PostgreSQL database before running the matrix. | `blocked-env` |
| `TestTwoWorkerContentionPostgreSQL` | `TRPC_RECOVERY_PG_DSN` unset; test needs the same isolated PostgreSQL setup. | `blocked-env` |
| `TestCrashAfterToolResult` | `TRPC_RECOVERY_GRAPH_PROVIDER` unset; this standalone case requires an executable real graph provider. | `blocked-env` |

The SQLite matrix builds and runs the repository's provider binary, and its HTTP counter is a deterministic local double. It is useful process-kill evidence for the repository path but does not prove a commercial model provider, provider-side query/idempotency semantics, production PostgreSQL, a production database, or a client delivery chain.
