# Craft T20 Container Boot Failure Diagnosis — 2026-10-06

## Context

This diagnosis was captured while executing the authorized Craft #107 final verification plan at `main` HEAD `6e1b2072a13a798e7ec25be44784e5242bd2f178`. The approved full-stack command is the documented harness in `docs/testing/craft/web-acceptance.md`; a bare `pnpm test:craft:web` is insufficient because its Playwright config depends on values provisioned by the harness.

## Failed acceptance attempts

- Bare `CRAFT_STACK_TAG=craft107-final-20261006 pnpm test:craft:web`: exited before running tests because `CRAFT_AUTH_STATE` was unset (`ENOENT` opening an empty path). This does not count as an acceptance run.
- Official `craft-stack.sh up mock`: the first attempt had a malformed inherited `PATH` entry containing whitespace and failed OpenCode readiness; no test run occurred.
- Official harness retry with a sanitized runtime `PATH`: OpenCode and model fixtures became ready, but the Go server never became ready and the harness tore down its run. Logs were retained under `/tmp/craft-craft107-final-20261006-mock.qnbC80/` during diagnosis.
- A separate manual server launch used the harness's actual distinct origins (`http://localhost` for the app, `https://127.0.0.1` for preview) and captured the fatal container error. A prior manual launch with both origins on `127.0.0.1` deliberately induced the preview-service same-host guard and was not the harness root cause; that earlier interpretation was retracted in the execution commentary.

## Proven root cause

The production container fails when `router.NewRouter` is first resolved:

```text
failed to build *gin.Engine: missing dependencies for function
"github.com/Tencent/WeKnora/internal/router".NewRouter:
missing type: *handler.CommercialWebhookHandler
```

Source inspection confirms `internal/container/container.go` registers `newCommercialWebhookService` and `newCommercialReconciliationService`, and `internal/router/router.go` requires `*handler.CommercialWebhookHandler` in `RouterParams`. `RegisterCommercialRoutes` uses that handler. The `handler.NewCommercialWebhookHandler(*commercialsvc.WebhookService)` constructor exists, but the production container has no `Provide` for it. Its dependency is already registered.

This is an application startup defect exposed by T20 integration verification. The focused implementation plan is `docs/superpowers/plans/2026-10-06-craft-container-webhook-provider-fix.md`; it requires a real-container regression test that resolves `*gin.Engine` before the one-line provider registration.

## Scope and limits

No production or test source was changed during diagnosis. The separate SQLite warning about a missing `commercial_reconciliation_state` table did not cause this fatal and is excluded from the repair absent further evidence. The approved Craft test, cleanup, and final OCR gates remain incomplete until rerun after the repair.
