# W15 batch and provider pause fix report

- Commit: `336ca52b` (`fix(notifications): support partial batches and provider pause`)
- Baseline: `b1d5ad70`

## Implemented

- Added an optional `NotificationBatchProvider` boundary with stable durable delivery IDs and per-item receipts/errors.
- Added Expo batch request/response handling. Successful receipts are acknowledged independently; failed or omitted items go through the existing fenced retry/expiry path.
- Added durable `mobile_notification_provider_state` storage for pause reason, alert count, pause time and recovery time, with SQLite `000061` and PostgreSQL `000139` migrations.
- Configuration failures pause the provider, leave the delivery pending for recovery, and prevent claim/retry storms while the provider remains unconfigured.
- A restarted worker clears the durable pause when its provider reports configured and records recovery before claiming new work.
- Wired the state store into the production notification worker composition root.
- Preserved the W13 revision fence for permanent device-token revocation.

## Evidence

- `go test ./internal/notification -count=1` — PASS.
- `go test -race ./internal/notification -count=1` — PASS.
- `go vet ./internal/notification` — PASS.
- Workbench focused batch/pause and existing notification tests — PASS when the unrelated W20 `remote_dispatch.go` field/method collision is temporarily corrected in the test checkout. The temporary correction was restored and is not part of this commit.
- Repository provider-state test — PASS under the same temporary W20 compile correction.
- `git diff --check` — PASS.
- Full workbench and migration suites remain `blocked-env` by the pre-existing W20 compile collision and SQLite migration 000055 transaction/version baseline failure; no full-suite or PostgreSQL runtime claim is made.
- Real Expo/APNs/FCM delivery and iOS/Android device evidence remain `blocked-env` because no provider credentials or devices were available.
