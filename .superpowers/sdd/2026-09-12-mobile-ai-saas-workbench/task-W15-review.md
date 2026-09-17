# W15 independent review — provider delivery, receipts, retry

- Worktree: `.sdd-worktrees/w15`
- Commits reviewed: `c62655dc` (implementation), `2ef77b60` (implementation evidence)
- Baseline: `927faf3d` (W14)
- Review scope: `docs/superpowers/plans/2026-09-12-mobile-workbench-03-notifications.md` W15 and its provider/worker/migration/config/container requirements.

## Verification evidence

- `go test ./internal/notification -count=1` — PASS.
  - Covers classification, local HTTP receipt parsing, 429 numeric `Retry-After`, permanent `DeviceNotRegistered`, and missing receipt.
- `go test ./internal/application/service/workbench -run 'Test(Push|NotificationWorker|NotificationDelivery|NotificationRetry)' -count=1` — BLOCKED by the pre-existing W20 compile collision in `internal/application/service/workbench/remote_dispatch.go` (`dispatch` field and method have the same name).
- `go test ./internal/config -run '^$' -count=1` — PASS.
- `git diff 927faf3d..c62655dc --check` — PASS.
- No live Expo/APNs/FCM credentials or devices were available; live provider evidence is `blocked-env` and is not counted as acceptance.

## Findings

### P0 — delivery workers are provided but never started in the production container

`internal/container/container.go` registers `NewNotificationWorker` and `NewNotificationDeliveryWorker` with `dig`, but there is no container invoke/start hook for either worker. A repository-wide search finds only the `Start` method definitions and unit-test calls; no production call to `NotificationWorker.Start` or `NotificationDeliveryWorker.Start` exists. Therefore an application can construct the services while no event projector or delivery loop consumes them. This fails W15 Step 4's “接通实际入口” and makes configured push delivery inert.

Required fix: add an explicit lifecycle invoke that starts both workers with the application context and registers shutdown/cleanup, while keeping provider failures non-fatal to HTTP startup. Add a container/lifecycle test proving `RunOnce` is reached or the worker is actually started under the production composition.

### P1 — permanent provider failures expire the delivery but never revoke the device registration

`NotificationDeliveryWorker.releaseDeliveryWithCause` calls `NotificationStore.Expire` for `ProviderError{Retry:false}`. It does not call `MobileDeviceStore.Revoke` (or any equivalent device invalidation port). W15 requires permanent token failures to revoke the device. The next event for the same invalid token can therefore enqueue and retry again. The implementation has no test asserting device revocation after `DeviceNotRegistered`/`BadDeviceToken`.

Required fix: inject a device-revocation port/store and perform an owner/tenant/device/environment/revision-fenced revoke on permanent token errors; test that the provider is called once, the delivery becomes terminal, and the device is inactive for subsequent claims.

### P1 — configured provider selection and credentials are not wired

W15 adds `internal/notification/expo.go` and `MobileNotificationConfig` (including `AccessToken`), but `newMobileNotificationProvider` always constructs `workbenchservice.NewHTTPNotificationProvider(endpoint)`. It never constructs `notification.NewExpoProvider`, never passes `AccessToken`, and has no provider-kind/config selection. The direct Expo implementation is unreachable from the production container; the loaded access token is dead configuration. This does not satisfy “SDK/HTTP供应商选择用配置” and makes the claimed provider contract only a local fixture path.

Required fix: define an explicit provider mode and wire the selected implementation through config. If the scoped gateway is the sole production provider, remove the dead Expo/config path or document and test the gateway contract, including its auth and receipt semantics.

### P1 — `Retry-After` is lost in the production HTTP gateway adapter

`internal/application/service/workbench/notification_delivery.go` handles HTTP 429 by creating `ProviderError{Code:"MessageRateExceeded", Retry:true}` without parsing the response `Retry-After` header. The worker's backoff honors `ProviderError.RetryAfter`, so the configured gateway path ignores the server retry hint. Only the standalone, currently unwired `internal/notification.ExpoProvider` parses it. Add a local HTTP gateway test with a 429 and both delta-seconds/date forms, and propagate the parsed bounded hint to `RetryAt`.

### P2 — no batch/partial-result provider behavior is implemented or verified

The W15 acceptance explicitly calls for “批量部分失败只重试失败条”. The provider contract and worker send exactly one `NotificationDelivery` per HTTP request and have no batch result type or per-item receipt/error handling. The existing tests do not exercise a partial batch. Either implement a batch provider contract with per-item result persistence or explicitly narrow the W15 contract and update the plan/spec before accepting the task.

### P2 — provider configuration failure has no alert/pause state

The W15 implementation report and code describe fail-closed behavior for an empty endpoint, but the worker simply retries the same row using normal backoff. There is no provider pause/health state or alert emission for invalid configuration. This is a required operational behavior (“配置错误告警并暂停该provider”), so it needs a durable/observable mechanism or an explicit documented boundary.

## Spec / quality decision

- **Spec: FAIL** — P0 lifecycle wiring and P1 permanent-token revocation/provider-selection gaps violate required behavior. The W15 task must not unlock W16/W31/W34 on this evidence.
- **Quality: CHANGES_REQUIRED** — focused provider tests are useful and pass, but the workbench integration suite cannot compile because of the unrelated W20 collision, and the missing production lifecycle path prevents end-to-end verification.
- **Provider/live evidence:** `blocked-env`; no real Expo/APNs/FCM delivery is claimed.

A new fix loop is required. After fixes, rerun the focused provider tests, the workbench worker tests once the W20 compile blocker is resolved, migration checks on SQLite/PostgreSQL, and an independent review of the changed lifecycle/provider/revocation paths.
