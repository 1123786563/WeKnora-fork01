# W14 targeted recovery fix report (round 3)

## Scope

- Added migration-backed notification projection tests with 257 valid runs,
  active mobile device bindings, durable intents, and checkpoints.
- Added a transaction rollback and restart/replay test. The first projection
  intentionally fails after a valid event to prove the intent and checkpoint
  are both absent; replay then commits one idempotent intent and cursor.
- Added a durable `budget_exhausted` event-to-intent integration assertion.
- Added an HTTP notification gateway adapter and registered the delivery worker
  in the container and server lifecycle. The adapter sends only the scoped
  tenant/owner/device/run identity; token ciphertext remains server-side.
- Added provider-spy coverage for valid delivery and revoked-device rejection,
  plus HTTP adapter request-shape and fail-closed endpoint checks.

## Validation

- `gofmt` and `git diff --check`: pass.
- Focused Go commands remain blocked before test execution by the unrelated
  W20 `remote_dispatch.go` field/method collision and missing dispatch-store
  methods. This is recorded as `blocked-env`; no package test result is claimed.
- Full migration-backed execution remains blocked by the existing SQLite
  migration 000055 `cannot start a transaction within a transaction` failure.
  The tests use `openRunTestDB`, which applies the real SQLite migration stream,
  and therefore fail closed rather than replacing production schema with a
  hand-written fixture.

## Remaining boundary

The HTTP adapter is a provider boundary and requires
`MOBILE_NOTIFICATION_PROVIDER_URL`; an empty URL fails closed and retries the
lease. Provider vendor credentials and native push SDK delivery remain a later
provider integration concern.
