# Ollama recovery provider evidence

Date: 2026-09-20. Provider model: local Ollama `gemma4:latest` at
`http://127.0.0.1:11434`.

The recovery provider now has an explicit opt-in path through
`TRPC_RECOVERY_OLLAMA_MODEL`. With that variable set it constructs the
repository's `chat.OllamaChat` and does not fall back to the scripted model.
The model contract requires exactly one initial call to the declared tool with
`{"tick":1}`, then a non-empty assistant answer with no further tool calls.

| Command | Result | Evidence classification |
| --- | --- | --- |
| `TRPC_RECOVERY_OLLAMA_MODEL=gemma4:latest OLLAMA_BASE_URL=http://127.0.0.1:11434 GOWORK=off go test ./internal/agent/recoverytest -run '^TestCrashMatrixSQLite$' -count=1 -v` | PASS, 10/10 cases, 130.04s | SQLite real-process/provider recovery PASS |
| `TRPC_RECOVERY_PG_DSN=... GOWORK=off go test ./internal/agent/recoverytest -run '^TestCrashMatrixPostgreSQL$' -count=1 -v` with the same Ollama variables, after `2473914c` | PASS, 9/9 cases, 138s | PostgreSQL + real Ollama recovery PASS |
| PostgreSQL matrix with the default scripted provider and the local disposable PostgreSQL 17.9 service | PASS, 9/9 cases, 47.76s | PostgreSQL durable path PASS, but not real-model evidence |

An earlier run failed because the harness reused the shared `/WeKnora`
database and a constant temporary namespace. Commit `2473914c` replaces any
PostgreSQL DSN database path and allocates unique database/schema names per
matrix. The corrected run is the authoritative result. P0 native Runner
product execution remains NO-GO only for the remaining persistence composition,
native product wiring and client/release acceptance gates.

Follow-up reproducibility check on 2026-09-20 with the same PostgreSQL and
Ollama settings did not settle the first two cases within the harness timeout
and was interrupted after 197s. This does not revoke the earlier 9/9 result,
but deterministic repeatability of the real-model PostgreSQL matrix is not
established; treat that row as a recorded pass with a flakiness caveat rather
than a fresh current acceptance run.
