# W15 batch provider error semantics fix

- Commit: `8fea8d17`
- Base: W15 chain ending at `ad3ff2d4`

## Changes

- `ExpoProvider.SendBatch` classifies HTTP 401 as `InvalidCredentials` and 403 as `InvalidProviderToken`, both non-retryable and non-revoking; recognized structured credential/configuration errors use the same durable-pause classification.
- `PushNotificationProvider.SendBatch` validates provider, resolver, and `Configured()` before invoking the batch adapter and fails closed with `InvalidProviderConfig`.
- Token resolver errors become per-delivery non-revoking configuration errors. Failed resolutions are omitted from the outbound batch, so an empty token can never revoke a valid device; valid deliveries retain their provider results.
- Added tests for 401/403/structured Expo errors, nil/unconfigured adapter, resolver error, and valid-device preservation.

## Verification

- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test ./internal/notification -count=1` — PASS
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test -race ./internal/notification -count=1` — PASS
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go vet ./internal/notification` — PASS
- `git diff --check` — PASS
- Focused `internal/application/service/workbench` tests could not compile because the pre-existing W20 collision remains in `remote_dispatch.go` (`dispatch` field and method share a name); no workbench runtime pass is claimed.
