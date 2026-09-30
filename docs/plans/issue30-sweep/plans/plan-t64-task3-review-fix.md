# T64 Task3 Review Fix — bounded cancellation reason and SQLite serialization

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Narrow repair plan for Task3 checkpoint `762ca4e40593beb86800a9b61914b4358ea209ba`.

**Goal:** Make the repository cancellation path robust to the full revocation reason and SQLite worker updates racing with cancellation.

**Findings:** R1 HIGH: full reason is written to PostgreSQL `agent_runs.wait_reason VARCHAR(64)` after the security ledger may already commit, so long reasons can roll back cancellation. R2 MEDIUM: candidate SELECT precedes first write, allowing SQLite read-to-write upgrade `SQLITE_BUSY` with concurrent worker update; current batch then fails.

## Global Constraints

- Work only in isolated T64 Task3 worktree based on `762ca4e40593beb86800a9b61914b4358ea209ba`.
- Owned files: `internal/application/repository/agent_run_security_cancel.go` and its test. No edits to shared security service, ledger, schemas/migrations, other repository lifecycle files, or unrelated tests.
- Keep complete reason in existing security ledger and cancellation event. Store a fixed bounded internal code in `wait_reason`; preserve tenant scope, terminal predicates, event, lease/revision, and session slot semantics.
- No live deployments, credentials or external writes. Local commit is authorized.

## Review Focus

1. Any legal non-empty reason length cannot cause PostgreSQL varchar overflow; bounded wait reason is explicit and event retains full reason.
2. SQLite writer reservation is established before candidate scan, or bounded full-transaction retry re-reads candidates after busy. No unbounded retry or partial batch.
3. Deterministic two-connection test interleaves a worker update with cancellation; no sleeps, assert committed final state and transaction atomicity.
4. Existing targeted cancel/lease/event/security tests remain green. PostgreSQL DSN is absent; state schema compatibility and test boundary honestly.

## Task 1 — repair cancellation persistence and concurrency

**Dependencies:** T64 Task3 original commit above; findings T64-T3-R1 HIGH and T64-T3-R2 MEDIUM.

**Role:** `backend_implementer`; independent backend validator and reviewer afterward. **Owned files:** exactly two files listed above.

1. RED: add a long-reason case proving cancellation state stores bounded wait_reason while event has full reason; add separate-connection deterministic SQLite worker-update/revocation interleaving test.
2. GREEN: use a fixed bounded security-cancellation wait reason. Acquire SQLite write reservation before scanning within tenant-scoped transaction (use existing supported repository/DB idiom); if portability requires transaction retry, cap attempts and retry the complete scan/write unit only for recognized busy failures.
3. Retain single atomic batch semantics and all existing per-run event/lease/session behavior.
4. Run exact targeted new tests, adjacent cancel/lease/event regression filters, package tests if practical, and `git diff --check`.
5. Self-review for non-SQLite backend behavior and ensure no source files outside ownership changed; commit locally and write independent report with commands/results.

**Failure handling:** If supported GORM dialect APIs cannot guarantee pre-scan SQLite writer acquisition safely, stop and report concrete constraint; do not paper over with sleeps or broad retries.
