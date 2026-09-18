# W15 independent batch/pause review

- Reviewed commits: `336ca52b`, `f6e4ae4e`
- Baseline: `b1d5ad70` (revision-fence fix chain)
- Scope: per-item batch success/failure, Expo batch provider, durable provider pause/alert/recovery and migrations, automatic resume, race/vet/tests, and retry-storm prevention.
- Reviewer: independent reviewer; no product code modified.

## Verification evidence

- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test ./internal/notification -count=1` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test -race ./internal/notification -count=1` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go vet ./internal/notification` — PASS.
- `git diff --check 927faf3d..f6e4ae4e` — PASS.
- `go test ./internal/application/service/workbench -run 'TestNotificationDeliveryBatchRetriesOnlyFailedItem|TestNotificationProviderPausePersistsAndRecovers|TestHTTPNotificationProviderPropagatesRetryAfter' -count=1` — **blocked before test execution** by the pre-existing W20 compile collision in `internal/application/service/workbench/remote_dispatch.go` (`dispatch` field and method have the same name). No workbench test execution is claimed.
- Static migration review: SQLite `000061_mobile_notification_provider_state` and PostgreSQL `000139_mobile_notification_provider_state` define the durable provider state table with pause reason, alert count, pause/recovery timestamps, and updated timestamp; filenames are ordered after the W15 delivery migrations.
- No PostgreSQL runtime migration evidence was available. The repository and service tests create equivalent tables ad hoc, so they do not prove the shipped migrations execute successfully.
- No live Expo/APNs/FCM credentials or iOS/Android devices were available. Live provider delivery remains `blocked-env`.

## Findings

### P1 — permanent provider errors ignore `ProviderError.Revoke` and can revoke devices for provider configuration failures

`internal/application/service/workbench/notification_delivery.go` in `releaseDeliveryWithCause` enters the permanent branch for every `ProviderError` with `Retry == false`, then calls `deviceRevoker.RevokeForTenant` whenever `Ack/Expire` succeeds. It does not require `providerErr.Revoke` to be true.

This is unsafe for the newly added provider classification: `InvalidCredentials`, `InvalidProviderToken`, and `MessageTooBig` are classified as non-retryable with `Revoke == false`, while `ExpoProvider` and the HTTP adapter can emit `InvalidProviderToken` for a 4xx response. A bad provider credential or malformed gateway response therefore expires the delivery and revokes the user's active registration. The `Revoke` field is otherwise explicitly part of the provider error contract and is set false for these cases.

Required fix: only revoke when `providerErr.Revoke` is true; retain a separate durable provider pause/alert path for provider-wide credential/configuration failures. Add a focused test for non-revoking permanent provider errors proving the delivery becomes terminal (or provider-paused according to the chosen policy) while the active device remains registered.

### P1 — malformed provider configuration is not paused and can retry indefinitely

`NewHTTPNotificationProvider` and `NewExpoProvider` only check that the endpoint string is non-empty. `send`/`SendBatch` return the raw `http.NewRequestWithContext` error for an invalid URL, rather than a `ProviderError{Code: InvalidProviderConfig, Retry:false}`. `releaseDeliveryWithCause` pauses the provider only for the exact `InvalidProviderConfig` or `mobile_notification_provider_unconfigured` codes.

Consequently an endpoint such as `://bad` is reported as an ordinary non-provider error; the worker calls `RetryAt` with exponential backoff on every eligible delivery and never writes the provider pause/alert state. A syntactically valid but unusable endpoint can also clear a previous pause because `Configured()` only checks `endpoint != ""`. This violates the configuration-error pause contract and leaves a retry storm over time.

Required fix: validate provider URLs at construction/config load, or normalize all request-construction/configuration failures to a non-retryable `InvalidProviderConfig`; make `Configured()` reflect the same validation. Add a repeated `RunOnce` test proving pause is durable and no new claims/provider calls occur while invalid configuration remains.

### P2 — integration evidence does not prove failed-item-only retry through the public worker loop

`TestNotificationDeliveryBatchRetriesOnlyFailedItem` exercises the private `runBatch` method with manually pre-claimed rows and checks only that one row is `sent` and one is `pending`. It does not call `RunOnce`, advance `next_attempt_at`, run the subsequent claim, or assert that the second provider call contains only the failed delivery ID. The per-item state transition is visible in code and the Expo unit test verifies a two-item partial response, but the required end-to-end retry selection remains unverified until the W20 compile blocker is repaired and this focused scenario is executed through `RunOnce`.

## Confirmed behavior

- `NotificationBatchProvider` maps stable durable delivery IDs to per-item receipts/errors.
- `ExpoProvider.SendBatch` sends an array and returns one `PushBatchResult` per response ticket; `runBatch` independently acknowledges successful receipts and routes omitted/failed items through the existing retry/expiry path.
- Provider state is durable and provider-keyed. `Pause` is idempotent and increments `alert_count` only on the healthy-to-paused transition; `Recover` clears pause on a subsequent worker pass when the provider reports configured.
- The paused gate runs before `Claim`, so rows are not re-claimed while a provider remains paused. Configuration failures keep rows pending for later recovery.
- The focused notification package race and vet checks pass. Workbench race/vet and provider pause/batch tests cannot execute until the unrelated W20 compile collision is repaired.

## Decision

- **Spec: FAIL** — the explicit operational contract is violated by the `Revoke` handling and malformed configuration path; the public-worker batch retry acceptance is also not yet evidenced.
- **Quality: CHANGES_REQUIRED** — focused provider package checks pass, but the workbench package is compile-blocked by W20 and the P1 safety/retry issues require implementation and tests.
- **Live provider evidence: blocked-env** — no real Expo/APNs/FCM delivery, revoked-token negative delivery, or native-device evidence is claimed.

W15 remains blocked and must not unlock W16/W31/W34 until the P1 findings are fixed and independently re-reviewed. After W20 repairs the package compile collision, rerun the focused workbench tests, race/vet, SQLite migration execution, and isolated PostgreSQL migration/runtime checks.
