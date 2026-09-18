# W15 independent error-semantics review

- Reviewed commit: `ad3ff2d4` (on the W15 chain through `336ca52b`, `f6e4ae4e`, and `b1d5ad70`)
- Baseline: `927faf3d` (W14 accepted chain)
- Reviewer: independent reviewer; no product code modified.
- Scope: `ProviderError.Revoke`, durable provider pause for credential/configuration failures, `Configured()` consistency, public `RunOnce` partial-batch retry, device preservation, and focused evidence.

## Verification evidence

- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test ./internal/notification -count=1` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test -race ./internal/notification -count=1` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go vet ./internal/notification` — PASS.
- `git diff --check 927faf3d..ad3ff2d4` — PASS.
- The requested focused `internal/application/service/workbench` tests cannot start because the checkout still has the unrelated W20 compile collision in `internal/application/service/workbench/remote_dispatch.go` (`dispatch` field and method have the same name). No workbench test pass is claimed.
- SQLite/PostgreSQL migration files are present and statically ordered as SQLite `000061` / PostgreSQL `000139`; no PostgreSQL runtime migration evidence is available.
- No live Expo/APNs/FCM credentials or native devices are available; live delivery remains `blocked-env`.

## Findings

### P1 — Expo batch top-level credential failures bypass the durable pause path

`internal/notification/expo.go:173-177` maps every non-429 non-2xx response from `SendBatch` to `ProviderError{Code: "UnknownTransport", Retry: true}`. A 401/403 from Expo caused by invalid credentials/provider token therefore enters `runBatch` as a retryable error. `releaseDeliveryWithCause` only pauses for non-retryable configuration codes (`InvalidCredentials`, `InvalidProviderToken`, `InvalidProviderConfig`, or `mobile_notification_provider_unconfigured`), so a batch worker keeps retrying and never writes `mobile_notification_provider_state`. The single-item `Send` path classifies 4xx differently, which makes behavior depend on batch size.

Required fix: classify batch 401/403 (and an explicit provider credential/config error in the response) as `InvalidProviderToken`/`InvalidCredentials` with `Retry:false`, `Revoke:false`, so the worker pauses durably. Add a public `RunOnce` test using a two-item batch and a 401/403 response that proves `paused=true`, the alert count is stable, rows remain pending, and no further provider call occurs until configured/recovered.

### P1 — batch token-resolution errors are converted to an empty token and can revoke a valid device

`internal/application/service/workbench/notification_delivery.go:117-121` discards the error returned while resolving a device token and sends an empty-token item to Expo. Expo may return `DeviceNotRegistered`/`InvalidRegistration`; the resulting `ProviderError` has `Revoke:true`, so `releaseDeliveryWithCause` can revoke the registration even though the failure was local token lookup/decryption/configuration rather than a confirmed invalid device token. This defeats the new `Revoke` guard and violates device preservation for provider/configuration failures.

Required fix: retain a per-delivery resolver error and return it as a non-revoking, retryable or provider-configuration `NotificationBatchResult` without submitting an empty token. Add a batch test with one resolver failure and one successful item proving the failed row is retried/paused according to its error class and the registration remains active.

### P2 — `Configured()` and batch send behavior are not fully consistent for an invalid adapter

`PushNotificationProvider.Configured()` correctly reports false when the provider or resolver is absent, but `SendBatch` does not validate either before invoking `p.resolve` (`notification_delivery.go:111-120`). A malformed construction with a nil resolver can panic in the public `RunOnce` batch path instead of returning `ProviderError{Code:"InvalidProviderConfig", Retry:false, Revoke:false}` and entering the durable pause path. The single-item path already has this guard.

Required fix: make `SendBatch` use the same configuration guard as `SendReceipt`; add a test for an invalid adapter with more than one claimed delivery.

## Confirmed behavior

- The new `releaseDeliveryWithCause` check only calls `RevokeForTenant` when `providerErr.Revoke` is true, and the existing revision-fenced revoke path is preserved.
- Malformed HTTP and Expo endpoint strings are rejected by `Configured()` and normalized to non-retryable `InvalidProviderConfig`; the focused test covers the malformed endpoint and durable pause path when workbench tests can compile.
- The public batch test now exercises `RunOnce`, forces the failed row eligible, and asserts only that row is sent on the second provider call. The test design is correct, but it cannot execute in this checkout because of W20's compile collision.
- Provider-state migrations include pause/recovery timestamps and `alert_count`; migration execution itself is not proven here.

## Decision

- **Spec: FAIL** — batch credential failures bypass durable pause, resolver failures can cause an unsafe revoke, and the requested workbench behavior is not executable in the current checkout.
- **Quality: CHANGES_REQUIRED** — notification package race/vet checks pass, but the two P1 safety/operational gaps require implementation and focused tests; rerun workbench race/vet and the public batch/pause tests after the W20 compile collision is repaired.
- **Live provider evidence: blocked-env** — no real provider or native-device delivery is claimed.

W15 must remain blocked and must not unlock W16/W31/W34 until the P1 findings are fixed and independently re-reviewed.
