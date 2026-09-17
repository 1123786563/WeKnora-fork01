# W15 targeted fix report

## Fixed

- The server composition already starts both notification workers; the desktop composition now starts and cancels both workers with its backend lifecycle.
- Permanent `ProviderError` failures expire the delivery and revoke the tenant/owner/device registration through the injected `MobileDeviceStore`.
- `MOBILE_NOTIFICATION_PROVIDER` / `mobile_notification.provider` selects `expo` or the scoped HTTP gateway. Expo receives `AccessToken` and resolves/decrypts the active device token only at send time.
- The HTTP gateway propagates both delta-seconds and HTTP-date `Retry-After` values into the bounded retry calculation.
- Provider and delivery focused tests cover retry hints and permanent-token revocation.

## Boundary

The current delivery contract is one durable delivery row per push request. It does not claim batch partial-result semantics; batch provider support needs a per-item receipt contract and will remain a follow-up. Provider configuration has no durable pause/alert state yet; invalid configuration remains fail-closed and retried by the existing outbox loop, which is an operational follow-up.

## Verification

- `go test ./internal/notification -count=1` PASS.
- `git diff --check` PASS.
- `go test ./internal/application/service/workbench` remains blocked by the pre-existing W20 `remote_dispatch.go` field/method name collision (`dispatch`).
