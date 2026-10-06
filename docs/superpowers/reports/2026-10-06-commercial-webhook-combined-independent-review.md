# Independent review — combined commercial webhook provider wiring and auth repair

## Scope and checkpoint

- Review type: read-only independent review of Task 1 plus Review Fix 1.
- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-webhook-provider/WeKnora-fork01`
- Frozen Git HEAD: `6e1b2072a13a798e7ec25be44784e5242bd2f178`.
- Delivery is an uncommitted four-file checkpoint. No source or test file was edited during this review.
- Reviewed files:
  - `internal/container/container.go`
  - `internal/container/bootsmoke/boot_smoke_test.go`
  - `internal/middleware/auth.go`
  - `internal/router/commercial_callback_public_test.go`

## Checkpoint integrity

The current source hashes matched the two task reports before and after the review:

| Task | File | Verified SHA-256 |
| --- | --- | --- |
| Task 1 | `internal/container/bootsmoke/boot_smoke_test.go` | `e56723e5f15c7953d39fa86c8ae799e8c73c461098017c779c15e7b78f00d23a` |
| Task 1 | `internal/container/container.go` | `51e083922d82feafe00237e68a8436ce27fdac49c2812af35d81b581848b7e21` |
| Review Fix 1 | `internal/middleware/auth.go` | `500e6c5c33ad7eccf0d45e87bea3f3b63fa64add379bb663715545a8e62e588c` |
| Review Fix 1 | `internal/router/commercial_callback_public_test.go` | `ecc95789875c1227e04e9ae85da4829be52ceabcddd48413f3ddbe307e86e810` |

The per-task binary patch hashes also match their recorded checkpoints:

- Task 1: `47793c658a15852ca42e5cd577347c607ed859df236973b9d52cc96f1e009ca4`
- Review Fix 1: `f074659c36e7c8077462e7e58b19f8f734cc7d2eff8adad0e738c61a684cf5b3`

`git diff --check` on all four reviewed files completed cleanly.

## Spec compliance — approved

The prior High finding is closed.

1. `BuildContainer` provides `newCommercialWebhookService` immediately before `handler.NewCommercialWebhookHandler`. The constructor consumes the produced `*commercialsvc.WebhookService`; `RouterParams` then receives the handler and passes it to `RegisterCommercialRoutes`. This repairs the missing DI edge without reordering a dependency after its consumer.
2. Global `middleware.Auth` is installed on the engine before the `/api/v1` route group. `RegisterCommercialRoutes` mounts the webhook on the parent `/commercial/webhooks` group, outside the commercial capability and billing-write groups. The endpoint therefore needs the global-auth exemption to reach its raw-body HMAC authentication boundary.
3. `noAuthAPI` permits exactly `POST` requests whose path begins `/api/v1/commercial/webhooks/`. The trailing slash prevents matching similarly named routes, and method membership rejects GET. The registered Gin route is the intended `POST /api/v1/commercial/webhooks/:provider` shape.
4. The assembled-router regression test mounts the real `Auth` middleware before `/api/v1`, then proves an anonymous webhook POST reaches the fail-closed handler (`503`) rather than Auth (`401`). Its two negative controls prove anonymous webhook GET and an unrelated commercial POST both remain `401`.

This matches the route's security model: external callers are anonymous to session/API-key Auth, while the handler validates the provider HMAC over the raw request body and fails closed for an unconfigured service/provider.

## Code quality — approved

- The container smoke assertion resolves the actual `*gin.Engine`, which exercises the router parameter graph and detects the original missing handler provider.
- The auth exception is concise, documents the authentication boundary, and uses the existing allowlist/method-matching mechanism.
- The test observes production middleware order and response behavior instead of only testing the allowlist helper. It includes a successful routing control and two authorization-boundary controls.

No critical, high, or medium code-quality findings were identified.

## Verification evidence

- `go test ./internal/container/bootsmoke -run TestBuildContainerBootsLite -count=1` — PASS (`2.399s`; linker emitted the existing non-fatal duplicate `-lc++` warning).
- `go test ./internal/router -run 'TestProviderCallbackRouteIsAnonymouslyReachable|TestCommercialWebhookRouteIsAnonymouslyReachable' -count=1` — PASS (`0.979s`).
- `go test ./internal/middleware -run TestIsNoAuthAPI -count=1` — PASS (`0.712s`).
- `ocr review --audience agent ... --exclude 'docs/**'` on the workspace scope — PASS, session `4dc62f73-9ddf-494e-9162-fa4f68108def`; OCR reported `0 finding(s) across 2 selected item(s)`. Raw OCR output: `/tmp/commercial-webhook-ocr-independent-review.txt`.

## Decision

**Approved.** The previous High authentication-blocking finding is resolved. This review found no new critical, high, or medium findings in the combined Task 1 and Review Fix 1 checkpoint.
