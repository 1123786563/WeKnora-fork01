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
| `TRPC_RECOVERY_PG_DSN=... GOWORK=off go test ./internal/agent/recoverytest -run '^TestCrashMatrixPostgreSQL$' -count=1 -v` with the same Ollama variables | FAIL: `after_admission` completed with `external_calls=0`; `after_plan_before_dispatch` did not reach its barrier and exited succeeded; run interrupted after 167s | PostgreSQL + Ollama recovery remains blocked; no pass claim |
| PostgreSQL matrix with the default scripted provider and the local disposable PostgreSQL 17.9 service | PASS, 9/9 cases, 47.76s | PostgreSQL durable path PASS, but not real-model evidence |

The failed Ollama/PostgreSQL run is retained as a blocker rather than being
converted into a skip. P0 native Runner product execution remains NO-GO until
the PostgreSQL real-provider matrix, persistence composition, client gates and
the remaining recovery acceptance are independently passing.
