# W15 independent batch/provider error review

- Reviewed commit: `8fea8d17` (W15 chain through `ad3ff2d4`, `f6e4ae4e`, `336ca52b`, and `b1d5ad70`)
- Baseline: `ad3ff2d4`
- Reviewer: independent reviewer; product code was not modified.
- Scope: Expo batch 401/403 and structured configuration error classification; `PushNotificationProvider` provider/resolver/configured preflight; empty-token prevention; resolver-error preservation; focused tests, race, vet, and diff checks.

## Verification evidence

- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test ./internal/notification -count=1` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test -race ./internal/notification -count=1` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go vet ./internal/notification` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test ./internal/application/service/workbench -run 'TestPushNotificationProviderBatch|TestNotificationDeliveryBatchRetriesOnlyFailedItem|TestNotificationPermanentProviderError|TestInvalidNotificationEndpoint|TestNotificationProviderPause' -count=1` — BLOCKED before test execution by the pre-existing W20 compile collision in `internal/application/service/workbench/remote_dispatch.go` (`dispatch` field and method share a name).
- The corresponding focused workbench race run is blocked by the same compile error; workbench race/vet PASS is not claimed.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go vet ./internal/application/service/workbench` — BLOCKED by the same W20 compile error.
- `git diff --check ad3ff2d4..8fea8d17` — PASS.
- No live Expo/APNs/FCM credentials or native devices were available; live delivery remains `blocked-env`.

## Confirmed behavior

1. `ExpoProvider.SendBatch` maps HTTP 401 to `InvalidCredentials` and HTTP 403 to `InvalidProviderToken`, and both classifications are non-retryable and non-revoking through `ClassifyPushFailure` (`internal/notification/expo.go:176-180`, `internal/notification/provider.go:53-63`).
2. Structured non-2xx response bodies are inspected for recognized configuration codes in `errors[].code`, a string `error`, or an object `error.code`; unknown responses remain `UnknownTransport` and retryable (`internal/notification/expo.go:210-252`).
3. `PushNotificationProvider.SendBatch` rejects nil provider, nil resolver, an explicitly unconfigured provider, and providers without the batch interface before resolving or sending (`internal/application/service/workbench/notification_delivery.go:111-120`).
4. Resolver errors are retained as per-delivery `ProviderError{Code:"InvalidProviderConfig", Revoke:false}` and wrapped with `%w`; the failed delivery is omitted from the outbound batch while valid deliveries retain their receipt (`notification_delivery.go:124-153`). The added test checks `errors.Is`, valid-item preservation, and no submitted item for the resolver-error case (`notification_delivery_test.go:54-76`).
5. The new Expo tests cover 401, 403, and a structured `errors[].code` response and assert non-retry/non-revoke semantics (`internal/notification/expo_test.go:103-130`).

## Finding

### P1 — a resolver that returns an empty token without an error still sends an empty token

`PushNotificationProvider.SendBatch` only handles `err != nil` and appends every successful resolver result, including `token == ""` or whitespace (`internal/application/service/workbench/notification_delivery.go:124-136`). `ExpoProvider.SendBatch` then converts an empty token into `expoMessage{}` and submits it (`internal/notification/expo.go:144-150`). Thus a missing/empty decrypted token, a buggy resolver, or a stale registration can still reach the provider as an empty message. The new test only covers `return "", resolverErr`; it does not cover `return "", nil`, and the implementation does not enforce the stated no-empty-token invariant at either seam.

Required fix: treat a trimmed-empty resolver result as a non-revoking provider/configuration result, omit it from the outbound batch, and ensure the direct Expo batch provider rejects or reports an empty item rather than serializing `{}`. Add a test with one resolver returning `"", nil` and one valid device that proves the provider receives only the valid item and the empty-token result is preserved without `Revoke`; add a direct Expo test if the provider remains responsible for validating batch items.

## Decision

- **Spec: FAIL** — the explicit no-empty-token invariant is not enforced for successful resolver results, despite the resolver-error path being fixed.
- **Quality: CHANGES_REQUIRED** — notification package tests/race/vet and diff checks pass, but the P1 safety gap requires a focused fix and independent re-review. Workbench package evidence remains blocked by the unrelated W20 compile collision.
- **Live provider evidence: blocked-env** — no real provider or native-device delivery is claimed.

W15 remains blocked and must not unlock W16/W31/W34 until the empty-token path is fixed and this review is rerun after the W20 compile blocker is repaired where possible.
