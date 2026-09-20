# PostgreSQL verification — 2026-09-20

Environment: disposable test schemas on the local PostgreSQL 17.9 container (`WeKnora-postgres-dev`, database `WeKnora`). The password-bearing DSN is intentionally omitted. Each test creates a random isolated schema and drops it during cleanup.

## Commands and outcomes

| Command | Outcome | Evidence |
| --- | --- | --- |
| `TRPC_TEST_POSTGRES_DSN=<isolated-local-dsn> GOWORK=off go test -race ./internal/application/repository -run 'TestNativeSchema|TestNativeSession|TestNativeMemory|TestNativeCommit|TestNativeEvent|TestNativeLease' -count=1 -timeout=300s` | PASS (`46.866s`) | PostgreSQL schema/migration, Session, Memory, CommitIntent/Event and lease repository tests ran with the PostgreSQL subtests enabled. |
| `TRPC_TEST_POSTGRES_DSN=<isolated-local-dsn> GOWORK=off go test -race ./internal/application/service -run 'TestNativeRecovery|NativeRecovery|NativeLease|NativeUsage|NativeBudget' -count=1 -timeout=300s` | PASS (`3.313s`) | PostgreSQL-enabled recovery/lease/usage service tests passed. |
| `GOWORK=off go test -race ./internal/agent/native ./internal/application/repository -run 'NativeMemory|NativeSession|NativeCommit|NativeEvent|Barrier|Reconcile' -count=1` | PASS | Native race regression remains green after the PostgreSQL run. |

The PostgreSQL server accepted connections with `pg_isready`, and reported PostgreSQL 17.9. No production schema or persistent application data was used by the tests.

## Remaining limits

The full repository suite is still outside this verification because it has a pre-existing duplicate-column migration failure in `TestArtifactVersionsMigrationDownIsReversible`. This report verifies the P1 native PostgreSQL paths above; it does not certify the full product or the P0 release gate.
