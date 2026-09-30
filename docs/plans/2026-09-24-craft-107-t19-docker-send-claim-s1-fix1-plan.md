# T19 Docker send-claim S1 fix1: cross-engine behavior proof

> **For Codex:** Close independent review Medium and Low test gaps with RED→GREEN, exact uncommitted checkpoint, independent re-review. No ExecStart, production wiring or commit.

**Sources:** `2026-09-24-craft-107-t19-docker-send-claim-s1-plan.md`, Task1 report/checkpoint and `2026-09-24-craft-107-t19-docker-send-claim-s1-review.md`. S1 persistence source matches contract; PostgreSQL migration down/up passed but receipt constraint/CAS/race behaviors lacked PG tests; bind write-failure/hold preservation lacked direct test.

## Global Constraints

Own only new `internal/application/repository/craft_docker_send_claim_test.go` and test report/checkpoint; production repository and migrations read-only unless tests expose a real defect requiring a separate scoped fix. Use disposable PostgreSQL 17 schema and SQLite; no shared DB. Keep `intent` hold/fence unresolved, no second send on replay/false inspect/404. Do not claim full T19 recovery.

## Review Focus

Same receipt/partial/duplicate constraints in PG and SQLite, two-connection CAS race one winner, stale revision, crash/reconstruction, failed Bind write leaves committed hold and no receipt/claim, SQL migration parity and exact task-local patch.

## Task 1

**Depends on:** S1 independent conditional review. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned file:** `craft_docker_send_claim_test.go` plus report/checkpoint.

1. RED: parameterize receipt constraint, bind/claim, two-connection race and stale-epoch cases to run under both SQLite and disposable PG. Inject bind write failure transaction (e.g. trigger/closed DB per engine) and assert prior intent/hold preserved, no `send_claimed_at`, no permission token.
2. Reuse existing repository API; do not adapt tests to hide PG behavioral differences. If PG source bug found, stop and ask for narrow ownership/review. Make engine/DSN setup deterministic and clean up disposable schema.
3. Run focused S1 suite on SQLite and PG17 plus race where valid, migration up/down/up, `git diff --check`; save exact pre/post test hash/patch, full commands/logs and independent re-review.

**Acceptance:** PostgreSQL receipt and CAS behavior has actual passing tests; bind failure cannot grant send or lose hold. **Failure handling:** unavailable PG is explicitly unverified, not PASS; no fake PG mock.
