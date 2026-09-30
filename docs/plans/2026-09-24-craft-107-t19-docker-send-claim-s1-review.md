# T19 Docker send claim S1 — independent review

Reviewed 2026-09-24 against the approved Craft spec (Task Budget and recovery), `CONTEXT.md`, ADR-0004, the Docker exec recovery design, the S1 plan, and the Task 1 report. Scope is the six implementation files in checkpoint `t19-docker-send-claim-s1-task1`; this is the repository prerequisite, not the complete T19 send/recovery path. No source, test, requirement, or remote issue was changed. OCR was not invoked.

## Findings

### S1-R1 — Medium — PostgreSQL behavioral acceptance is still unverified

- **Evidence:** `craft_docker_send_claim_test.go:62-148` runs receipt constraints, immutable binding, single claim, concurrent separate-connection race, replay, stale identity, and write failure only through `openRunTestDB`, which selects SQLite unless the test name contains `/postgres`. Only the migration subtest at lines 178-196 uses PostgreSQL. The worker report recorded `TRPC_TEST_POSTGRES_DSN` unset. I ran that PostgreSQL migration subtest against a local PostgreSQL 17 container using the helper's disposable schema; it passed, but it does not exercise PostgreSQL claim behavior or constraints.
- **Affected:** `TestCraftDockerSendClaimSchemaConstraints`, `TestCraftDockerSendClaimConcurrentOwnerRace`, and related repository tests.
- **Impact:** The requirement to verify receipt constraints and one-owner CAS under real PostgreSQL concurrency remains open. A passing SQLite race cannot establish PostgreSQL behavior, so cross-engine acceptance should not be marked verified.
- **Smallest correction:** Add PostgreSQL variants of the receipt-shape/uniqueness, bind/claim/replay, failure, and concurrent separate-connection tests using `openPostgresRunTestDB`; run them with `TRPC_TEST_POSTGRES_DSN` against a disposable schema and record the results.

### S1-R2 — Low — failed receipt-bind write has no direct behavioral test

- **Evidence:** `TestCraftDockerSendClaimFailureAndStaleEpochNeverAuthorizeSend` at lines 149-171 injects a trigger only for `UPDATE OF send_claimed_at`. It tests a stale bind revision, but no injected bind write error. The S1 plan Step 3 and Review Focus 4 call for failed bind **and** claim writes to preserve the original `intent`/hold/fence and grant no send permission.
- **Affected:** `craft_docker_send_claim_test.go`, failure test.
- **Impact:** The bind failure branch is straightforward and returns the SQL error, but its rollback/hold behavior lacks direct regression evidence.
- **Smallest correction:** Add an injected `BEFORE UPDATE OF provider, container_id, exec_id` failure, assert bind fails, receipt stays null, claim returns no permission, and the journal `intent` and reservation remain.

## Spec and quality verdict

**S1 design/code conforms to the reviewed persistence contract, subject to R1 verification.** The SQL checks/triggers reject partial or empty receipts and orphan claims; both engines use a partial unique receipt index. `BindDockerExecReceipt` conditionally writes only an empty receipt for the exact tenant/Run/activity/revision in `intent`, while exact replay is idempotent. `ClaimDockerExecSend` conditionally sets `send_claimed_at` once for the exact receipt and returns send authority only for one affected row. A claimed row stays in `intent`; replay returns `false`, and there is no claim-reset or Inspect-derived resend API. No transaction spans provider I/O in this slice. I found no source-code defect within the assigned S1 boundary.

**Quality gate: conditional.** R1 is a missing required cross-engine behavioral gate; R2 is a narrower test omission. S1 should not be recorded as fully verified until R1 is closed. The repository API alone does not establish physical Docker Start counting, daemon-restart recovery, or coordinator Run fencing; those are downstream tasks under the design and S1 plan.

## Evidence reproduced

- The checkpoint manifest SHA-256 values match all six current files and the patch; its snapshot contains exactly those six files and matches their bytes. The patch includes both ignored SQL migration pairs.
- `go test ./internal/application/repository -run '^TestCraftDockerSendClaim' -count=1 -v` passed all SQLite tests; its PostgreSQL subtest skipped with `TRPC_TEST_POSTGRES_DSN unset`.
- With `TRPC_TEST_POSTGRES_DSN` supplied from the local PostgreSQL 17 container and the test helper's disposable schema, `go test ./internal/application/repository -run '^TestCraftDockerSendClaimMigrationUpDownUp/postgres$' -count=1 -v` passed. It exercises full migration stream then S1 down/up and journal-row retention; it does not cover PostgreSQL constraints or claim races.
