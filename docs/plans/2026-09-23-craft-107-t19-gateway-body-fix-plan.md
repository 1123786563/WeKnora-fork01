# T19 Model Gateway Response Lifetime Fix Plan

> **For Codex:** Execute with SDD RED → GREEN → REFACTOR, exact uncommitted checkpoint and independent Spec/quality re-review. No commit.

**Goal:** Resolve High finding 1 in `t19-model-gateway-start-review.md`: a successful upstream `Do` must retain its bounded request context until the response body is read and closed; ambiguous body failures must remain unknown, not replayable.

**Sources:** Spec #107/#138, T19 durable-start plan Task3, model gateway first review/report, `CraftBudgetService.StartBinding` callback semantics, Go HTTP request context lifetime.

**Global Constraints:** Own only model gateway handler/test. Do not modify the budget coordinator or container assembly during this bounded fix. Commit-before-send and one-shot activity journal/hold must remain. Upstream response body must close on every success/error path, including a StartBinding error after `Do` returned a response. Keep existing body size and timeout bounds. Activity ID production propagation is a separate High finding and still blocks Task3.

**Review Focus:** delayed/streamed body bytes after headers, callback/context lifetime, body-close on all paths, unknown outcome after response body failure, no second external send on same activity key.

## Task 1 — keep body within bounded physical attempt

**Depends on:** first T19 gateway review FAIL High. **Role:** backend_implementer. **Owned files:** `internal/handler/craft_model_gateway.go`, `craft_model_gateway_test.go`, report/checkpoint. **Produces:** response bytes/error outcome with correct request context lifetime.

1. RED real HTTP test: server flushes headers, sends body later; current Forward fails due callback/coordinator cancel, test must demonstrate. Add failing-body/close and same-key retry tests.
2. GREEN: ensure the full upstream HTTP response body is consumed and closed while the StartBinding callback's bounded context remains active (or use an equivalent bounded context whose cancel happens after body read). Preserve response bytes for downstream forwarding outside callback. Any post-send body error reports unknown outcome to the coordinator and usage ledger; do not mark started with missing body. Close response on early StartBinding error.
3. Run focused handler and race tests, existing SQLite StartBinding tests, diff check and exact full-content checkpoint; independent reviewer rechecks High 1. Do not claim full Task3 until stable activity ID reaches real OpenCode calls.

**Failure handling:** If the coordinator callback contract cannot represent body-read unknown, report the exact incompatibility and stop for a contract amendment; do not silently relabel it confirmed or retry.
