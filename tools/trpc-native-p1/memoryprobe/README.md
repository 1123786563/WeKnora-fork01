# Native P1 memory SQL probe

This standalone module probes the exact `v1.11.0` composition of the root
memory API and its separately versioned SQLite and PostgreSQL backends. It is
evidence only; it does not wire a service into WeKnora.

Run the behavior characterization against SQLite and an isolated PostgreSQL
schema (each test creates and drops its own schema):

```sh
P1_MEMORY_PG_DSN='postgres://postgres@127.0.0.1:32768/postgres?sslmode=disable' \
  GOWORK=off go test -race -count=20 -timeout=5m ./...
```

Without `P1_MEMORY_PG_DSN`, PostgreSQL subtests report `blocked-env`; SQLite
still runs. `P1_REQUIRE_CONTRACT=1` enables the deliberately failing
generation/tombstone acceptance test. Its failure is the current native API
characterization, not a passing governance assertion.

The probe checks namespace isolation and reopen persistence; idempotent add;
metadata round-trip; update, delete and clear; 20 concurrent identical adds;
and SQLite connection ownership. It also records that raw `AddMemory` can
resurrect content after a delete or clear. The native API carries no generation,
tombstone, authorization revision, or commit-time CAS input, so a WeKnora
adapter needs those fields and an atomic conditional write before it can accept
stale extraction jobs.
