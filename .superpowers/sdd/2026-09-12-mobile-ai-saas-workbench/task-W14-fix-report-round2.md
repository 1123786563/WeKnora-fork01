# W14 targeted recovery fix report

## Commit

`8f638f0a fix(notifications): page discovery and atomically checkpoint projection`

The accepted W13 device implementation and migrations were integrated in the
same worktree before this fix (`446cbb10`, `b9a50a51`, `a149c9c2`, `0525c28c`,
`0ad1f631`).

## Changes

- Added `EventRunKeysPage` with a `(tenant_id, run_id)` lexicographic cursor;
  `NotificationWorker.RunOnce` now drains every page and cannot starve runs
  after the first 256.
- Added `ProjectEventsAndCheckpoint`, which projects a page and advances its
  consumer checkpoint in one GORM transaction. The worker uses this method;
  a crash before commit replays the page and a committed page always has its
  cursor.
- Added a 257-run pagination regression test.
- Added `NotificationDeliveryWorker`; it calls `RevalidateDelivery` directly
  before the provider `Send` and retries invalidated or failed leases.
- Classified durable run budget denials as `budget_exhausted`, allowing the
  existing event-to-notification policy to create the mobile intent.
- Updated SQLite migration expectations and required notification tables to
  version 59. W13 migrations 000058/000136 are present in both streams.

## Validation

- `git diff --check`: PASS.
- SQLite migration tests were run with `DEVELOPER_DIR=/Library/Developer/CommandLineTools`.
  They remain blocked by the pre-existing migration 000055 nested transaction
  failure (`cannot start a transaction within a transaction`), before W13/W14
  migrations execute.
- Go focused tests could not build because of the unrelated W20
  `remote_dispatch.go` field/method collision and missing dispatch-store
  methods. No Go runtime pass is claimed.

## Remaining acceptance boundary

Provider spy race coverage and a fresh migration/container smoke test still
require the W20 compile repair and a working database runtime. This report
does not claim PostgreSQL, native Expo, or real provider evidence.
