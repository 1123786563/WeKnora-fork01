# Native V03 experiment environment

This is a bounded, non-production fixture for the existing Go native graph
path. It owns only resources whose names start with `semantica-v03-native-`.
It uses dedicated loopback-only PostgreSQL, Redis, and Neo4j/APOC containers;
the running Go server is started on a distinct loopback port. It never loads
the repository `.env`, connects to existing dev containers, downloads models,
or uses a user database.

Copy `config.example.json` to an untracked local config, then prepare and
start it:

```sh
python3 semantic/experiments/native_env/orchestrate.py prepare --config /path/to/native-v03.json
python3 semantic/experiments/native_env/orchestrate.py up --config /path/to/native-v03.json
```

`up` writes random experiment-only credentials into
`artifact_dir/run_id/runtime.env` with mode `0600`; do not commit it. It starts
`go run ./cmd/server` with `NEO4J_ENABLE=true`, the fixed local Ollama model
`qwen2.5:0.5b`, short request/task/query limits from the config, and isolated
Postgres/Redis/Neo4j endpoints. It does not create an Ollama model, call paid
providers, or apply any production configuration.

The stdlib `NativeEnvClient` is the V03 evaluator-facing adapter. `run_case`
and `query` return `case_id`, `document_revision`, `requested_mode`,
`actual_mode`, engine/model version fields, IDs supplied as evidence scope,
native SSE references, latency, token field, status, and error. A failed native
request keeps `actual_mode: native`; an unstarted environment is an orchestration
blocker, not a fabricated fallback result. The caller must provision the
synthetic user, local model, graph-only KB and two Chinese manual documents,
then wait for terminal graph indexing before invoking a case. `knowledge-search`
is deliberately not used because it does not exercise the graph branch.

When binding the model through the initialization endpoint, include its enabled
`nodeExtract` configuration in that same request. The current handler disables
an existing extract config when `nodeExtract.enabled` is omitted; the adapter
must verify the persisted graph capability before publishing documents.

For the current bounded smoke, capture separate cold indexing and first-query
timestamps in the run metadata. Do not treat timings as V03 benchmarks while
another experiment shares Ollama; obtain an exclusive window before recording
cold/hot p50/p95 results. Actual token data is only usable when the server's
model response exposes it and its accounting invariant is validated; otherwise
it remains unavailable.

Teardown affects only the recorded run's process and its compose project:

```sh
python3 semantic/experiments/native_env/orchestrate.py teardown --config /path/to/native-v03.json
```

It calls compose `down --volumes` only after the run-prefix metadata check. The
report must retain endpoints (without credentials), image digests, command
exits, request/SSE evidence and any startup/indexing/query failure. A successful
smoke is native actual-mode evidence only; it is not Semantica, A03 gateway,
commercial usage, or production acceptance.
