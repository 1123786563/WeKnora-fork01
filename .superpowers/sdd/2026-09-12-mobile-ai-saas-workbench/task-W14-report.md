# W14 implementation report — transactional event to notification outbox

- Task: W14
- Worktree: `.sdd-worktrees/w14`
- Base: `4e9a512b`
- Status: implementation complete; independent review pending
- Scope: durable notification intent repository, event-kind policy/projector, lease/fence claim/ack/retry, SQLite/PostgreSQL migrations, focused tests.

## Migration ruling

The task text names `000125_mobile_notifications` and `000045_mobile_notifications`, but those versions already belong to the existing craft migrations in this checkout. Replacing either would make the migration stream ambiguous and can destroy an applied schema. W14 therefore uses the next migrations after the accepted W13 device migrations: `000137_mobile_notifications` and `000059_mobile_notifications`. The coordinator must preserve this ruling when integrating W13 and record the resulting integrated versions.

## Implementation

- `NotificationIntent` is keyed by tenant/event/owner/device/environment and persisted idempotently.
- `Claim` leases only unexpired intents whose device is active and whose run still belongs to the owner; every claim increments attempt and fence.
- `Ack` and `Retry` require worker plus fence, preventing stale workers from acknowledging or requeueing a newer claim.
- `NotificationEventKind` allows only terminal, interaction, and budget events; token events are ignored.
- `NotificationStore.ProjectEvent` fans one durable event to current active devices and can be replayed after a crash.
- `NotificationProjector` reads the durable run event stream and returns a cursor; callers persist the cursor after the idempotent projection transaction.

## Verification

- `git diff --check`: PASS.
- SQLite migration `000059_mobile_notifications.up.sql` applied in a temporary SQLite database and schema inspected: PASS.
- RED before implementation: `go test ./internal/application/repository -run TestNotificationOutbox -count=1` failed with undefined `NewNotificationStore`/`NotificationIntent`: PASS as behavior RED.
- Focused Go tests after implementation: BLOCKED by pre-existing W20 compile errors in `internal/application/service/workbench/remote_dispatch.go` (`dispatch` field/method collision and missing dispatch methods); the failure occurs before W14 tests execute.
- PostgreSQL migration and runtime acceptance: BLOCKED-ENV; no `TRPC_TEST_POSTGRES_DSN` was available.
- Provider/mobile native delivery: out of W14 and deferred to W15/W16; no provider or native evidence claimed.
