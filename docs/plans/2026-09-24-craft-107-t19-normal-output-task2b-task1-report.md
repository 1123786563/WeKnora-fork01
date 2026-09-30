# T19 Normal Output Task2b Task1 Report

## Scope

Implemented Task 1 from `2026-09-24-craft-107-t19-normal-output-task2b-plan.md`. Changed only the new normal-input repository and tests, the Docker send-claim repository seam, and SQLite 125/PostgreSQL 204 migrations. No coordinator, provider, output store, R4/R5 code, production routing, or commit was changed.

## Behavior

- `CraftDockerNormalInputRepository.Stage` validates and persists a create-only canonical request keyed by tenant/Run/activity before any Docker create. It serializes with the current Run row, checks the current tenant-bound Task/session, executable status, matching journal revision, and coordinator intent state. Exact replays remain readable/idempotent after bind; changed command, environment, stdin, timeout, output quota, or output policy conflicts.
- The full canonical request (including command, environment and stdin) is encrypted with the existing deployment `SYSTEM_AES_KEY` through the project AES-GCM helper. This repository path explicitly rejects a missing or invalid 32-byte key and verifies the encrypted prefix; it never falls back to plaintext and does not generate or invent keys. Read verifies decryption, canonical request digest, stdin length and stdin digest. Empty stdin keeps a distinct enabled flag and the SHA-256 of the empty byte sequence.
- Normal receipt bind persists provider/container/exec IDs plus stdin enabled/count/hash and timeout. Normal claim requires that exact staged full receipt. The legacy three-field bind/claim methods refuse any activity that already has a normal-input stage, preserving the outputless path without allowing it to authorize normal execution. SQLite and PostgreSQL constraints/triggers enforce scope, receipt completeness, input/receipt equality, journal receipt equality, and immutability.
- Database and cryptographic errors do not include request bytes. Receipt and input mismatch errors are conflict/corruption categories without sensitive data.

## TDD and verification

RED was established with tests that initially failed to compile because the normal-input API and receipt seam did not exist. GREEN verification at the final source/test hashes:

- `TRPC_TEST_POSTGRES_DSN=<dedicated local PG17 DSN> go test ./internal/application/repository -run '^TestCraftDockerNormalInput' -count=1 -v` — PASS for SQLite and PostgreSQL. PostgreSQL uses a unique temporary schema with minimal current Run/journal parent tables, applies actual migration 204, exercises stage/replay/scope/receipt/concurrent stage behavior, and runs 204 up/down/up. It does not depend on the blocked full pgvector migration chain.
- `TRPC_TEST_POSTGRES_DSN=<dedicated local PG17 DSN> go test -race ./internal/application/repository -run '^TestCraftDockerNormalInput' -count=1` — PASS for SQLite and PostgreSQL.
- SQLite and PostgreSQL tests cover exact replay; divergent command/env/stdin/timeout/quota/policy conflicts; fail-closed key handling; encrypted-at-rest bytes; corruption detection; foreign Task/tenant/Run rejection; zero-byte identity; full receipt bind and immutable replay; restricted receipt/claim rejection for staged normal operations; concurrent exact staging; and migration up/down/up.
- `gofmt` and `git diff --check` on changed Go files — PASS.

Saved focused and race outputs are referenced by the checkpoint. Migration SQL is ignored by the repository `.gitignore`; all four migration files are included in the task-local patch and hash manifest.

## Limits

- This Task 1 exposes repository seams only. The coordinator/service caller integration and one-start Docker behavior remain for later plan Tasks 2 and 3; no physical Docker execution or production route was enabled here.
- PostgreSQL verification intentionally applies migration 204 in an isolated schema with minimal parent tables rather than claiming the full migration chain passes; the project notes that full-chain setup requires the pgvector base extension.
- No expiry/deletion policy is implemented for the encrypted staged blob in this Task. Its confidentiality depends on operators preserving `SYSTEM_AES_KEY` for recovery and protecting that deployment secret. Key loss/rotation makes staged input unavailable and fails closed.

## Checkpoint

See `2026-09-24-craft-107-t19-normal-output-task2b-task1-checkpoint.json` and `2026-09-24-craft-107-t19-normal-output-task2b-task1-task-local.patch`. The patch was applied to a reconstructed S2 Fix1 preimage; all eight resulting task-file hashes matched this checkpoint.
