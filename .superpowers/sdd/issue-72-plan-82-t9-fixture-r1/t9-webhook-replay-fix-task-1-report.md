# Task 1 Report — COMPLETE (offline evidence)

**Date:** 2026-09-30
**Frozen BASE:** `561aee31524483077941e9981a5fcc363327e714`
**Lago API source pin:** `591ae9005110346f1c6034ec72ea9046625668cf`
**Workspace:** `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue-72-82-t9-fixture-r2`

## Changes

- Added exact-event inbound webhook SQL read using `exec.CommandContext` and argument-vector Docker/psql invocation. Query is scoped by organization UUID, Stripe source, provider code, and payload event ID using psql `:'variable'` quoting. Baseline requires exactly one succeeded row; replay requires exactly one additional row and polls until succeeded under a 90-second child deadline before any post-replay object reads.
- Updated T9 prep script to export active DB service container ID plus effective PostgreSQL user/database labels without printing the values.
- Added the full canonical projection: exact active purchase subscription; exact finalized+succeeded invoice from PaymentIntent metadata; all payment pages filtered to exact invoice ID with exactly one succeeded payment; all customer wallet pages; all transaction pages for every wallet. Every collection requires `meta.total_count` completeness, validates stable Lago IDs, sorts by those IDs, and marshals normalized JSON for byte comparison.
- Retained the authority snapshot and succeeded payment count assertions as additional checks. Added isolated helper tests for baseline/replay states, missing/malformed/duplicate/failed rows, stable sorting, and incomplete pagination.
- No production Go source changed. No live services were started.

## Verification

- `gofmt -w internal/modules/commercial/commercialplatform/lago_settlement_integration_test.go` — PASS
- `go test -tags=lago_integration ./internal/modules/commercial/commercialplatform -run 'TestInboundWebhookReplayGateAndCanonicalCollection' -count=1` — PASS
- `go test ./internal/modules/commercial/commercialplatform -count=1` — PASS (`72.660s`)
- `go test -tags=lago_integration ./internal/modules/commercial/commercialplatform -run '^$' -count=1` — PASS (compile-only; no tests run)
- `bash -n docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh` — PASS
- `git diff --check` — PASS

Focused build-tag-free selector had no tests to run because the helper lives in the env-gated integration test file; the tagged selector above is the effective helper test.

## Hashes and live status

- Go integration test SHA-256: `153b9a2729f695dbe5dea3a8e6e323d4594948a89400baee059a661a0b5e5f02`
- T9 prep script SHA-256: `916df717b75bb6318c0dfd0803830e85c0cd7466c44979f6bfa869e9db8090ee`
- Implementation commit: `b7751ee51`
- The replay assertion was **not run live**. AC3/T9 live acceptance and AC4 real Alipay sandbox evidence remain OPEN; compile-only is not live evidence.

## Fix round — pinned API contract corrections

- Changed payment selection to require exact invoice membership in Lago `invoice_ids`; malformed/non-string association shapes fail closed. Added match/exclusion/malformed tests.
- Changed psql to receive query text on stdin via `psql -f -`; psql variables remain separate argv values and safely quoted by `:'name'`.
- Pagination envelopes now decode `meta` separately from the named collection array. A failed webhook row now returns immediately instead of polling to timeout.
- Snapshot validates that the purchase wallet (name prefixed by the exact external purchase identity) exists and is active; all wallet objects and transactions remain included.
- T9 prep resolves DB user/database defaults before the organization query and uses those same labels for that query.
- Fix round SHA-256: Go `ce94bfcdbc8c89346e206db8484368fc575756915c58fdc3d67803d4cc1c851d`; prep script `6057ec4067a3602f5eb86d6483a723ba05dc704a4890e4efbaadc4b435417c7d`.
- Verification: tagged focused selectors (payment invoice membership, webhook gate/canonicalization, immediate failed-row handling) PASS; full package `-count=1` PASS (`72.213s`); integration-tag compile-only PASS; `bash -n` PASS; `git diff --check` PASS.
- No live services started and no live replay claimed. Live T9/AC3 and AC4 remain OPEN. Previous implementation/report commits remain intact; this fix round is committed separately.
