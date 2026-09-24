# T19 Docker Send Claim S1 Fix1 Task 1 Report

## Result

Closed the independent review gaps with tests only. The receipt constraint, idempotent bind, immutable receipt collision, stale revision, durable replay, concurrent claim, bind-write failure, and claim-write failure behaviors now run under both SQLite and PostgreSQL. No production source or migration changed; no source defect was found.

The injected bind failure leaves the journal at `intent`, preserves its prepared `run_revision` and dispatched reservation, leaves receipt and claim fields null, and a subsequent claim returns no permission. The injected claim failure preserves the same unresolved journal and hold after an exact receipt is bound. Each engine's CAS test uses two separately opened database connections and observes exactly one `claimed=true` result.

No Docker `ExecStart` was invoked. This is still only the S1 repository persistence prerequisite, not full T19 recovery.

## Changed files

- `internal/application/repository/craft_docker_send_claim_test.go`
- `docs/plans/2026-09-24-craft-107-t19-docker-send-claim-s1-fix1-task1-checkpoint.json`
- `docs/plans/2026-09-24-craft-107-t19-docker-send-claim-s1-fix1-task1-checkpoint.patch`
- `docs/plans/2026-09-24-craft-107-t19-docker-send-claim-s1-fix1-checkpoint/preimage/craft_docker_send_claim_test.go`
- `docs/plans/2026-09-24-craft-107-t19-docker-send-claim-s1-fix1-checkpoint/postimage/craft_docker_send_claim_test.go`
- `docs/plans/2026-09-24-craft-107-t19-docker-send-claim-s1-fix1-checkpoint/verification.log`
- `docs/plans/2026-09-24-craft-107-t19-docker-send-claim-s1-fix1-pre.json`
- This report

Only the assigned test file was changed from the implementation write set. No S1 production code, SQL migration, or other worker's files were changed.

## PostgreSQL 17 test setup and cleanup

The actual server was `PostgreSQL 17.9 (Debian 17.9-1)`. A one-use superuser login and database with generated names/password were created for the run because the repository's full historical PG migration stream creates extension-owned base types. Tests used that database with the existing helper, which creates a separate `trpc_test_*` schema per test and drops it on cleanup. A shell exit trap terminated residual DB connections and removed the one-use database and login after the command.

An initial attempt with a non-superuser on the shared default `postgres` database failed in historical migration `000002` at extension type creation. That test role and the extension/schema state it introduced in the default database were removed. The final successful run used a disposable database instead. Post-cleanup inspection reported zero `codex_t19_s1_%` roles, zero `codex_t19_s1_db_%` databases, and zero `pg_search` extensions in the default `postgres` database.

## Verification evidence

Commands run in `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`:

| Command | Result |
| --- | --- |
| `go test ./internal/application/repository -run '^TestCraftDockerSendClaim' -count=1` | PASS on SQLite; PostgreSQL subtests skipped when no DSN was supplied |
| `TRPC_TEST_POSTGRES_DSN="postgres://${PG_TEST_ROLE}:${PG_TEST_PASSWORD}@127.0.0.1:5432/${PG_TEST_DB}?sslmode=disable" go test ./internal/application/repository -run '^TestCraftDockerSendClaim' -count=1 -v` | PASS, 23.910s, with actual PostgreSQL 17.9 and SQLite subtests both passing |
| Second disposable PostgreSQL 17 DSN: `go test ./internal/application/repository -run '^TestCraftDockerSendClaimConcurrentOwnerRace$' -count=5` | PASS, 23.909s; each run exercises SQLite and PostgreSQL separate-connection races |
| Included in the focused suite: `TestCraftDockerSendClaimMigrationUpDownUp` | PASS on SQLite and PostgreSQL; down/up retained the existing journal row |
| `gofmt -d internal/application/repository/craft_docker_send_claim_test.go` | PASS, no output |
| `git diff --no-index --check /dev/null internal/application/repository/craft_docker_send_claim_test.go` | PASS, clean output |

The full no-credential test transcript is saved at `docs/plans/2026-09-24-craft-107-t19-docker-send-claim-s1-fix1-checkpoint/verification.log`; DSN values, generated role/database names, and passwords are omitted. The successful focused PG run included the PostgreSQL receipt partial/duplicate constraints, exact and divergent binds, one-owner race, stale epoch and identity rejection, bind-write and claim-write trigger failures, hold preservation, crash/reconstruction replay, and migration down/up. Test output contained two GORM `SLOW SQL >= 200ms` notices for disposable-schema cleanup (`DROP SCHEMA ... CASCADE`, 207ms and 225ms); the tests passed.

## Checkpoint

- Base HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`.
- Preimage SHA-256: `962b6b04a714cf88c41e52ce8a208b79eb04abbfd09f3f701324461c7635031c`.
- Postimage SHA-256: `1499dd9f7ed99c2de2d76d60565a19c3c8fd9d581bd92395d6f6e345fc38ae15`.
- The checkpoint manifest records both byte counts, preimage/postimage paths, the SHA-256 of the exact test-only unified patch, and the verification-log hash.
- No commit was created.

## Review handoff and remaining limits

The S1-R1 evidence gap is closed for PostgreSQL 17.9 and SQLite. The S1-R2 bind failure case now directly proves that no send claim is available and the dispatched hold remains. Root should run the planned independent re-review against the exact fix1 checkpoint. No broader T19 recovery, Docker physical send behavior, or provider daemon-restart behavior is claimed here.
