# Task 1 independent review — container webhook provider

## Scope and frozen checkpoint

- Reviewed baseline: `6e1b2072a13a798e7ec25be44784e5242bd2f178`.
- Reviewed uncommitted production/test diff only:
  `internal/container/container.go` and
  `internal/container/bootsmoke/boot_smoke_test.go` (6 added lines, no
  deletions).
- Source hashes verified against the Task 1 report:
  - `internal/container/bootsmoke/boot_smoke_test.go` —
    `e56723e5f15c7953d39fa86c8ae799e8c73c461098017c779c15e7b78f00d23a`
  - `internal/container/container.go` —
    `51e083922d82feafe00237e68a8436ce27fdac49c2812af35d81b581848b7e21`
- Scoped patch SHA-256 verified:
  `47793c658a15852ca42e5cd577347c607ed859df236973b9d52cc96f1e009ca4`.
- Read-only review; no source or test files were changed.

## Spec compliance — changes requested

### High — the newly mounted provider webhook route is still blocked by global authentication

`handler.NewCommercialWebhookHandler` makes `router.NewRouter` mount
`POST /api/v1/commercial/webhooks/:provider` through
`RegisterCommercialRoutes` (`internal/router/routes_commercial.go:160-165`).
That route is explicitly anonymous and authenticated by its HMAC.  It is
registered after the global `middleware.Auth` middleware
(`internal/router/router.go:308-310`), so it must appear in `noAuthAPI`.

The allowlist contains the analogous payment callback wildcard only
(`internal/middleware/auth.go:73-80`), and its focused test includes that
callback but no webhook entry (`internal/middleware/auth_noauth_test.go:14-23`).
Therefore an actual provider POST without a session reaches `Auth` first and
returns 401 before `CommercialWebhookHandler.HandleWebhook` can verify its
signature. The handler provider repair restores boot but leaves its intended
webhook contract unavailable in production.

Repair the scoped behavior by adding the webhook POST wildcard to the no-auth
allowlist and asserting the anonymous webhook path in its middleware test;
then re-run the focused router/container/handler checks against a new frozen
checkpoint and send it through review again.

## Code quality — changes requested

The provider registration itself is correctly positioned: its constructor
consumes the `*commercialsvc.WebhookService` supplied immediately before it,
and `router.NewRouter` is only provided later in `BuildContainer`. No extra
provider or service behavior was added.

The `*gin.Engine` resolution in `TestBuildContainerBootsLite` is a meaningful
regression seam: resolving the real container output instantiates
`router.NewRouter`, whose `RouterParams` requires the webhook handler, so it
would fail on the exact missing DI type. The non-nil assertion is harmless
but redundant after `dig.Container.Invoke` succeeds.

The seam is incomplete for the actual request path: it proves router
construction only, while the high-severity authentication failure remains
outside the assertion. This is the sole code-quality finding and must be
addressed together with the spec-compliance repair above.

## Decision

Rejected pending the high-severity route-authentication repair. No other
critical, high, or medium findings were identified in the frozen two-file
diff.
