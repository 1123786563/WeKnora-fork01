# Task 1 Report — T9 PaymentIntent fixture identity

## Scope

Implemented only the assigned #82 T9 test-harness repair. No production source changed. Worktree: `.worktrees-issue72/issue-82-t9-fixture`; branch `codex/issue-72-82-t9-test-fixture-r1`; BASE `8329b85d4dfff301d03f94406dfc829d87cb5b26`.

## Changes

- Added deterministic helper coverage in the existing `lago_integration`-tagged test file. It verifies an old succeeded PI is rejected, the captured candidate is selected after success, unlinked intents are excluded, and no-match diagnostics include expected and observed IDs.
- The bounded pre-settle wait captures IDs only from invoice-linked `requires_payment_method`, `requires_action`, or `requires_confirmation` rows. Expiry fails with HTTP status and observed IDs.
- The synthetic post-settle event now uses only a succeeded row whose ID was captured before settle and whose invoice metadata is nonempty. Historical succeeded rows are not fallback candidates.
- Updated `docs/plans/issue-72-ledger-82.md`, `docs/plans/issue-72-dag.md`, and `docs/plans/issue-72-execution-ledger.md` with evidence and residual status.

## TDD and verification

The pure regression was added before wiring the live harness to the candidate set. It is build-tagged alongside the live test; the focused selector runs only the helper test and does not execute any network/live integration case.

Commands and results:

1. `gofmt -w internal/modules/commercial/commercialplatform/lago_settlement_integration_test.go` — passed.
2. `go test -tags lago_integration ./internal/modules/commercial/commercialplatform -run 'Test.*PaymentIntent.*(Candidate|Selection)' -count=1` — PASS, package reported `ok` (3.330s).
3. `go test ./internal/modules/commercial/commercialplatform -count=1` — PASS (72.210s).
4. `go test -tags lago_integration ./internal/modules/commercial/commercialplatform -run '^$'` — PASS; tagged test package compiled, no tests ran (0.273s).
5. `git diff --check` — PASS, no diagnostics.

No Docker, PostgreSQL, network API, or live payment command was run.

## Acceptance and residuals

The fixture cannot select an unrelated pre-existing succeeded PI. The captured candidate succeeds selection after settle; missing linkage fails closed with diagnostic expected/observed IDs. Production code is unchanged.

The dedicated T9 live-stack rerun remains incomplete because its dedicated `lab.env`/credentials are unavailable. AC4 real Alipay sandbox evidence remains unavailable. These gates are retained as incomplete in the #82 durable ledger; local unit/package/compile evidence does not claim them complete.

## Review and commit

Implementation self-check and targeted verification are complete. The task is committed locally on the assigned branch; the final HEAD is reported to the parent. Independent review/validator status is pending parent dispatch.
