# W14 fix report

- Base: `773c8269`
- Fix head: `0ce9a045`
- Scope: production notification projector wiring, durable event identity validation, claim revalidation, and crash checkpoints.

## Changes

- Added `NotificationWorker`, durable event-run discovery, monotonic per-tenant/run checkpoints, and server-context startup wiring.
- Registered `NotificationStore`, `NotificationProjector`, and `NotificationWorker` in the production dig container; the worker starts with the HTTP server context and retries without advancing a cursor on failure.
- `ProjectEvent` now loads `agent_runs.owner_id` and `agent_run_events.event_type` inside the same transaction, rejects cross-owner, nonexistent-sequence, and type-mismatch requests, and fans out only to active devices.
- Added `RevalidateDelivery`, the delivery worker's final authorization fence immediately before provider send; it conditionally checks the leased row, active device, and current owner/run relation.
- Added `mobile_notification_checkpoints` to SQLite `000059` and PostgreSQL/versioned `000137` migrations.
- Added focused durable identity and monotonic checkpoint tests.

## Verification

- `gofmt` — PASS.
- `git diff --check` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test ./internal/application/repository -run 'TestNotification' -count=1` — BLOCKED-ENV/BASELINE before test execution by unrelated W20 compile errors in `internal/application/service/workbench/remote_dispatch.go` (field/method `dispatch` collision and missing dispatch-store methods).
- PostgreSQL runtime evidence — BLOCKED-ENV (`TRPC_TEST_POSTGRES_DSN` unavailable).

The fix is committed as `0ce9a045`. A fresh independent review is required before W15/W16 are unlocked.
