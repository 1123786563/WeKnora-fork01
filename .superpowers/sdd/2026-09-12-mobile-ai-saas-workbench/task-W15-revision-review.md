# W15 revision-fence independent review

- Worktree: `.sdd-worktrees/w15`
- Reviewed commit: `b1d5ad70` (`fix(notifications): fence permanent device revocation`)
- Compared with W15 accepted implementation chain ending at `927faf3d`
- Reviewer scope: claim-time `DeviceRevision`, permanent-provider-error/rebind race, notification tests, and the remaining batch/pause requirements.

## Verification evidence

- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test ./internal/notification -count=1` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test -race ./internal/notification -count=1` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go vet ./internal/notification` — PASS.
- `git diff --check 927faf3d..b1d5ad70` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test ./internal/application/service/workbench -run 'TestNotificationDeliveryPermanent|TestHTTPNotificationProvider|TestNotificationRetryBackoff' -count=1` — BLOCKED before test execution by the pre-existing W20 compile collision in `internal/application/service/workbench/remote_dispatch.go` (`dispatch` field and method share a name). No workbench test pass is claimed.

## Revision-fence review

The reviewed fix correctly adds `DeviceRevision` to `repository.NotificationDelivery`, captures the active registration revision in `NotificationStore.Claim`, and passes that exact revision to `RevokeForTenant`. `MobileDeviceStore.RevokeForTenant` uses the revision as a compare-and-swap condition, so a rebind between provider failure and revoke returns `ErrMobileDeviceRevision` and does not revoke the replacement row.

The new `TestNotificationDeliveryPermanentFailureDoesNotRevokeReboundRegistration` directly exercises the intended interleaving: claim revision 1, bind a replacement, then process the permanent provider failure. The test asserts the CAS conflict and that revision 2/token-v2 remains active. The basic permanent-revocation test remains present as well. The code-level fence and focused test are therefore **PASS**, subject to the workbench package compile blocker preventing execution in this checkout.

## Remaining acceptance gaps

### P1 — batch partial-result retry is still absent

The W15 specification requires a batch provider response with one result per item and retry persistence for only failed items. `NotificationDelivery` remains one durable row per push, `PushProvider.Send` accepts one token/payload, and `ExpoProvider` still selects the first ticket/receipt. There is no stable per-item receipt model, partial-result persistence, or failed-item-only retry path. This is an explicit acceptance requirement, not an optional follow-up.

### P1 — provider configuration errors are still retried without alert/pause

An empty endpoint returns `mobile_notification_provider_unconfigured`, but the delivery worker routes it through the ordinary retry path. There is no durable provider health/paused state, alert/audit signal, claim gate, or restart/recovery test. A malformed provider configuration can therefore generate repeated retries rather than pausing the provider and surfacing an operator-visible failure. This is also an explicit W15 acceptance requirement.

### Environment limitations

- The W20 `remote_dispatch.go` compile collision prevents running the focused `workbench` notification tests in this branch.
- PostgreSQL/migration-backed runtime evidence is unavailable in this review environment; the claim-time query and CAS behavior are static/code-level evidence plus the new focused test, which could not be executed because of the package compile blocker.
- No real Expo/APNs/FCM credentials or iOS/Android device was available. Live delivery and invalid-token behavior remain `blocked-env`.

## Decision

- **Spec: FAIL** — revision fencing is fixed, but the required batch partial-result retry and provider configuration alert/pause behavior remain unimplemented.
- **Quality: CHANGES_REQUIRED** — notification package tests, race tests, vet, and diff checks pass; the workbench test suite is blocked by W20 and the remaining operational gaps need implementation and tests.
- **W15 remains blocked** and must not unlock dependent notification tasks until the two explicit acceptance gaps are implemented or the authoritative W15 specification is amended and re-reviewed.

