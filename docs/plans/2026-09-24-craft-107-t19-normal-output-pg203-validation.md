# T19 Normal Output PostgreSQL 203 Validation

Date: 2026-09-24
Scope: isolated migration/runtime DDL proof for `migrations/versioned/000203_craft_docker_output.{up,down}.sql` only. No production source, tests, or shared integration files were modified.

## Environment and isolation

- PostgreSQL server: `17.11 (Debian 17.11-1.pgdg13+2)`.
- Database: parent-provided disposable local `craft107_t19` database at `127.0.0.1:32771`; credentials were supplied through `TRPC_TEST_POSTGRES_DSN` and are not copied into this report.
- Each run created a unique `t19_output_fix1_<timestamp>` schema and configured the connection search path to only that schema.
- Created only minimal `agent_runs` and `craft_charge_start_journal` prerequisite tables with the composite keys and columns referenced by migration 203. The complete migration chain was not run because preceding R4/R5 migrations were still being integrated.

## Command

From the integration Worktree:

```sh
TRPC_TEST_POSTGRES_DSN='<parent-provided isolated PG17 DSN>' go run /tmp/verify_t19_pg203.go
```

The temporary verification program used `golang-migrate` with `Force(202)` and `Steps(1)`, ran the checks below, then `Steps(-1)` and `Steps(1)` again. It dropped the unique schema in a defer and checked `pg_namespace` afterward.

## Result

```text
PostgreSQL server_version=17.11 (Debian 17.11-1.pgdg13+2) current_schema=t19_output_fix1_1790211687879699000
PASS schema=t19_output_fix1_1790211687879699000 migration=203 up/down/up x2; receipt validation, immutable identity trigger, zero-byte quota metadata, and input length constraint exercised
cleanup PASS schema=t19_output_fix1_1790211687879699000 removed
```

Both up runs reached version 203 with `dirty=false`, created the operations/chunks tables, and exposed `input_byte_count` and `input_sha256`. Both down runs removed the tables.

The runtime DDL probes verified:

- A valid operation matching the persisted Run task and already-claimed Docker receipt inserts successfully through the scope trigger.
- A wrong task for an otherwise persisted claimed activity is rejected by the scope trigger.
- Updating the immutable Exec identity is rejected by the identity trigger.
- A zero-byte stored chunk with original nonzero input length and `input_truncated=true` inserts successfully.
- A chunk with `input_byte_count < byte_count` is rejected by the database check constraint.

The first harness attempt failed before migration execution because it derived the repository root from its `/tmp` source file and selected `file:///migrations/versioned`. The harness was corrected to use the integration Worktree working directory and rerun; the recorded run above passed. The one-off verification source remained in `/tmp` and was not added to the repository.

## Limits

This validates PostgreSQL 17.11 syntax, migration reversibility, and the listed schema triggers/checks in an isolated minimal schema. It does not run repository Go tests against PostgreSQL, the full migration chain, or production service append/seal operations. No application data or shared schema was touched.
