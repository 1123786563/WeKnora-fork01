# Commercial Webhook Anonymous Authentication Repair Plan — Review Fix 1

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to execute this plan. Preserve the uncommitted-checkpoint policy; do not commit.

**Goal:** Let anonymous commercial provider webhook POSTs reach the webhook handler so HMAC verification is the authentication boundary, while keeping other commercial endpoints behind global session/API-key authentication.

**Finding:** Independent Task 1 review `docs/superpowers/reports/2026-10-06-craft-container-webhook-provider-task1-review.md` found a High Spec-compliance defect. `router.NewRouter` applies `middleware.Auth` to `/api/v1` before `RegisterCommercialRoutes` mounts `/commercial/webhooks/:provider`. `internal/middleware/auth.go:noAuthAPI` exempts `/api/v1/commercial/callbacks/*` only. The webhook route comment and `CommercialWebhookHandler` contract explicitly require anonymous reachability with raw-body HMAC verification; therefore current anonymous provider calls return 401 before the handler.

**Authority:** Live Issue #107/#139 and T20 execution mapping, approved Craft Spec and commercial callback/webhook endpoint contract. Finding evidence is source-backed in the named Review report.

**Worktree:** `/Users/wuyongjun/.codex/worktrees/craft-webhook-provider/WeKnora-fork01`, continuing the uncommitted provider registration checkpoint at HEAD `6e1b2072a13a798e7ec25be44784e5242bd2f178`. Source hashes consumed: `internal/container/container.go` `51e083922d82feafe00237e68a8436ce27fdac49c2812af35d81b581848b7e21`; `internal/container/bootsmoke/boot_smoke_test.go` `e56723e5f15c7953d39fa86c8ae799e8c73c461098017c779c15e7b78f00d23a`.

**Commit policy:** No staging, commits, push, merge, deploy, or Issue mutation. Capture only scoped uncommitted diff, snapshot hashes, and report.

## Global Constraints

- Fix only the authenticated reachability gap. Do not broaden exemptions beyond `POST /api/v1/commercial/webhooks/*`.
- Keep `/api/v1/commercial/orders` and unrelated commercial methods authenticated.
- Add the regression before changing the whitelist. The public-route proof must run the real `middleware.Auth` against the real registered webhook route and distinguish handler fail-closed `503` from middleware `401`.
- Do not change handler HMAC logic, webhook payload semantics, migrations, or provider service behavior.
- The previous provider Task 1 source/test diff is frozen; this task owns only `internal/middleware/auth.go` and `internal/router/commercial_callback_public_test.go`, plus its report.
- Validator and reviewer are read-only and bind conclusions to exact source hashes.

## Review Focus

- Anonymous webhook POST reaches the handler and gets fail-closed `503` when the handler is deliberately unconfigured, proving middleware did not reject it first.
- Only webhook POSTs gain the exemption; GET on that route and unrelated commercial POSTs remain 401 for anonymous callers.
- The existing anonymous payment callback tests still pass.

## Task 1 — Test the regression, then add the method-limited exemption

**Depends on:** finding verified against frozen provider Task 1 checkpoint.
**Role:** backend_implementer.
**Validator:** backend_validator, read-only.
**Owned files:** `internal/router/commercial_callback_public_test.go`, `internal/middleware/auth.go`; task report under `docs/superpowers/reports/`.
**Consumes:** authenticated group and route wiring from `internal/router/router.go` / `routes_commercial.go`; existing `middleware.Auth` and `CommercialWebhookHandler` fail-closed response.
**Produces:** test proving anonymous webhook reaches its handler; narrow `noAuthAPI` exemption; evidence report.

1. Verify current provider Task 1 source hashes and record task-before snapshot.
2. Extend `TestProviderCallbackRouteIsAnonymouslyReachable` or add a sibling test that mounts `middleware.Auth` before `/api/v1`, then registers the real commercial webhook route with `handler.NewCommercialWebhookHandler(nil)`.
3. Assert anonymous `POST /api/v1/commercial/webhooks/lago` reaches the fail-closed handler and returns 503, not 401. Add negative controls: anonymous `GET` on the webhook path returns 401 and anonymous `POST /api/v1/commercial/orders` returns 401.
4. Run the new test before production edits and capture RED: webhook POST returns 401.
5. Add only `"/api/v1/commercial/webhooks/*": {"POST"}` to `noAuthAPI`, with a comment stating the HMAC-signature boundary and the direct `/api/v1` group behavior.
6. Rerun the public webhook route test and existing `TestProviderCallbackRouteIsAnonymouslyReachable`; both must pass. Run `go test ./internal/middleware -run TestIsNoAuthAPI -count=1` and `go test ./internal/router -run 'TestProviderCallbackRouteIsAnonymouslyReachable|TestCommercialWebhookRouteIsAnonymouslyReachable' -count=1`.
7. Run scoped `git diff --check`, hash owned files, write exact RED/GREEN outputs and verify the previous provider checkpoint remains unchanged.

**Acceptance:** TDD test is 401 before the exemption; becomes 503 at the unconfigured handler afterward; negative controls remain 401; all targeted tests pass; only owned files change.
**Failure handling:** if current router setup cannot exercise production middleware, do not replace it with `isNoAuthAPI` unit-only coverage; report the concrete seam issue and update the plan first.

## Task 2 — Independent validation and Review

**Depends on:** Task 1 frozen checkpoint.
**Role:** backend_validator, then independent reviewer (read-only).
**Owned files:** validation/review reports only.

1. Check exact source and diff hashes, rerun the focused tests read-only.
2. Review Spec compliance and code quality separately, confirming both positive and negative controls and the precise POST-only prefix.
3. Return any defect to a scoped SDD repair before integrating the checkpoint.

**Acceptance:** validator passes against the exact checkpoint; both independent review verdicts pass; no unrelated files enter the final integration tree.

## Dependency and resource preflight

| Work | Depends on | Files/interfaces | Shared resources | Dispatch |
|---|---|---|---|---|
| Anonymous route regression + whitelist | verified review finding | middleware auth whitelist + router HTTP test; existing global-auth/route contract stable | Go cache only | one backend implementer |
| Validation and Review | frozen Task 1 checkpoint | read-only same source hashes | Go cache | concurrent independent reads |
| Full Craft browser stack | both SDD fixes pass and integrate | full Craft path | exclusive Docker, SQLite, nginx, test ports | only after both review gates |
