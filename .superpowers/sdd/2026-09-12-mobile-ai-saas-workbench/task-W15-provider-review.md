# W15 independent provider fix review

- Worktree: `.sdd-worktrees/w15`
- Fix commit: `76f2d6e6bd28181e152a6507517edcf058a19f26`
- Baseline: `927faf3d` (W14 accepted chain)
- Review scope: worker lifecycle, permanent provider-error device revoke, Expo/provider selection and access token, Retry-After propagation, focused tests, and the remaining W15 acceptance requirements.

## Verification evidence

- `go test ./internal/notification -count=1` — PASS.
- `go test ./internal/application/service/workbench -run 'TestHTTPNotificationProvider|TestNotificationDeliveryPermanentProviderErrorRevokesDevice|TestNotificationRetryBackoff' -count=1` — BLOCKED at package compile by the pre-existing W20 `remote_dispatch.go` field/method collision (`dispatch`). No W15 test execution is claimed from this command.
- `git diff --check 927faf3d..76f2d6e6` — PASS.
- `cmd/server/main.go` already starts both notification workers under the server context; `cmd/desktop/main.go` in the fix starts and cancels both workers with the desktop backend lifecycle. This satisfies the lifecycle wiring at those two composition roots, subject to runtime evidence.
- No live Expo/APNs/FCM credentials or devices were available. Live delivery remains `blocked-env`.

## Findings

### P1 — permanent revocation is not revision fenced

`internal/application/service/workbench/notification_delivery.go:245-247` calls `RevokeForTenant(..., 0)` after a permanent provider error. A zero revision means “revoke whichever active row exists now.” The delivery intent carries no registration revision, so if the device is rebound between the provider call and this revoke, the worker can revoke the newer registration that replaced the failed token. W13's device store deliberately exposes revision CAS for this race, but this path bypasses it.

Required fix: capture the registration revision in the durable intent/delivery or add an equivalent compare-and-swap identity, and pass that expected revision through the revoker. Add a test that rebinds the device after claim/provider failure and proves the replacement registration stays active.

### P2 — required batch partial-result behavior is absent

The W15 plan requires “批量部分失败只重试失败条”. The provider boundary and worker still process one durable delivery per request. `ExpoProvider.firstTicket` selects only the first ticket from an array, and the HTTP adapter returns one receipt/error; there is no per-item result model, persistence, or retry selection. The fix report explicitly leaves this as a follow-up. This is an explicit acceptance item, so it cannot be silently narrowed during review.

Required fix: either implement a batch provider contract with stable per-item delivery IDs and persistence/retry for only failed items, or update the authoritative W15 spec before claiming acceptance.

### P2 — provider configuration errors are not alerted or paused

An empty endpoint or unknown provider mode is converted to a provider that returns `mobile_notification_provider_unconfigured`; the delivery loop then applies ordinary retry backoff. No durable provider health/pause state, alert event, or operator-visible notification is emitted. The fix report explicitly states that configuration errors remain retried and that pause/alert is follow-up, while the W15 plan requires “配置错误告警并暂停该provider”. This leaves a bad deployment continuously retrying and does not meet the stated operational contract.

Required fix: add a durable/provider-scoped paused state with an alert or audit signal, make config errors stop new claims for that provider, and provide focused tests for pause, alert, recovery, and restart behavior; or amend the authoritative spec before acceptance.

## Confirmed fixed behavior

- The server and desktop composition roots start the projection and delivery workers using cancellable application contexts.
- `MOBILE_NOTIFICATION_PROVIDER` / `mobile_notification.provider` selects the direct Expo adapter or the scoped HTTP gateway; the Expo path receives the configured access token and resolves/decrypts the active device token at send time. The focused tests and static wiring do not prove live provider delivery.
- HTTP 429 responses propagate both delta-seconds and HTTP-date `Retry-After` values into `ProviderError.RetryAfter`; the worker applies the bounded retry calculation.
- Permanent provider errors expire the delivery and invoke the injected device store. The focused revocation test proves the basic active-device case, but not the revision race above.
- Provider failures do not synchronously abort HTTP startup; the worker retries on subsequent ticks.

## Decision

- **Spec: FAIL** — the explicit batch partial-result and configuration-error alert/pause requirements remain unimplemented, and permanent revocation is not fenced to the failed registration revision.
- **Quality: CHANGES_REQUIRED** — focused provider tests pass, but the workbench suite is compile-blocked by W20 and the remaining concurrency/operational gaps need tests and implementation before W15 can unlock W16/W31/W34.
- **Live provider evidence: blocked-env** — no real mobile delivery is claimed.

The next fix loop should address the P1 revision fence first, then either implement or formally change the batch and provider-pause contract. Re-run the focused workbench tests after W20's compile collision is repaired and repeat independent review.
