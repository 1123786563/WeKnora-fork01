# T19 model gateway body-lifetime fix — independent re-review

Date: 2026-09-23. Read-only review of the two owned handler files against the first model-gateway review High finding 1, `t19-gateway-body-fix-plan.md`, its checkpoint/report, durable-start Task 3, and #138/T19 acceptance. No source/test edits or OCR.

## Checkpoint and verdict

The live owned source and test SHA-256 match the saved checkpoint: `internal/handler/craft_model_gateway.go` `c2d754451fbed3d9e9aace619665892ecebc7e228c946f2eed96d0ce6dcc47d7`; `internal/handler/craft_model_gateway_test.go` `a13c6ece2f9eec8dca350dfc3ead1447c7ec8d0e5d8dc88f273320a895af5e4f`. Checkpoint patch SHA-256 also matches `34fbfcb3ce67bf224220ef0effd1310c34d3cc647f39c4b4f858c656535143e6`.

- **Scoped Spec compliance: FAIL.** The specific premature cancellation on successful `Do` is fixed: `responseCtx` remains active while the body is read and closed. The fix plan additionally requires post-send body errors to become coordinator `unknown`; the gateway still commits `started` when only headers arrive, then detects body errors afterward. The separate activity-ID High finding remains open.
- **Scoped code quality: FAIL.** The response lifetime and `(response,error)` close path improved, but the independent request context no longer obeys the coordinator's 30-second initiation deadline. New tests use an injected transport, not a delayed real HTTP server, and do not assert journal state after body failure.
- **Full Task 3/T19: NOT VERIFIED.** Production activity-ID propagation, sandbox starts and joined acceptance remain outstanding.

## Findings

### 1. High — body failure is recorded as a resolved `started` charge start

**Evidence / symbol:** `CraftModelGateway.Forward`, `internal/handler/craft_model_gateway.go:507-520,544-558`. The callback returns `CraftChargeStartStarted` as soon as `Do` returns a response. `CraftBudgetService.StartBinding` persists that as `state='started'` before returning (`internal/application/service/craft_budget.go:311-325`). Only afterward does the handler discover nil body, read failure, over-limit body or `Close` failure and record nil usage. The lifecycle fence treats only `intent` and `unknown` as unresolved (`internal/application/repository/agent_run.go:532-537`), so `started` no longer holds the Run in reconciliation. The fix plan explicitly requires a post-send body failure to report `unknown` to the coordinator.

**Impact:** A call with lost response bytes is considered resolved for Run transitions, despite unknown provider/usage outcome. Retry with the same key is rejected, but downstream work can proceed through a fence that should remain unresolved. The usage ledger's nil observation does not repair the journal state.

**Smallest defensible correction:** Keep the response read/close within the bounded start attempt and return `Unknown` on any ambiguous body outcome, or add a durable compare-and-swap outcome amendment before permitting Run transitions. Assert actual journal state and hold/fence behavior for read, close and oversize failures in a service-backed gateway test.

### 2. Medium — `Do` ignores the coordinator's initiation timeout

**Evidence / symbol:** `CraftModelGateway.Forward`, `internal/handler/craft_model_gateway.go:491-513`; `CraftBudgetService.StartBinding`, `internal/application/service/craft_budget.go:293-299`. The request uses `responseCtx` with `g.timeout` (default five minutes) and derives it from the inbound context. The callback checks `startCtx.Err()` once, then passes `req` with the unrelated context to `g.forward`. The coordinator's 30-second cancellation therefore cannot interrupt a blocked `Do`.

**Impact:** A stalled provider can keep the initiation callback and unresolved start in flight for up to the longer gateway timeout; the intended short initiation bound is not enforced. Current tests do not verify deadline propagation during `Do`.

**Smallest defensible correction:** Enforce the start deadline on the `Do` operation while preserving the response context through body completion. A transport-level bound or a carefully separated initiation/body context is acceptable; add a stalled-`Do` test that observes the coordinator deadline without canceling a healthy delayed body.

## Positive evidence and verification

`responseCtx` is no longer canceled at callback return, and the body is read with a `craftMaxForwardBody+1` limit and explicitly closed. Both `startErr` and non-started result paths close a returned body. Same-key retry remains rejected by the test fake. The delayed-body test observes an active request context after the callback and succeeds after releasing bytes. It is an injected `Forward` function rather than a real `net/http` server, so transport behavior is still unverified.

I independently ran `go test ./internal/handler -run 'TestCraftGateway|TestCraftModelActivityKey' -count=1` (pass, 1.652s), `go test -race ./internal/handler -run 'TestCraftGateway|TestCraftModelActivityKey' -count=1` (pass, 3.117s), `go test ./internal/application/service -run '^TestCraftChargeStart' -count=1` (pass, 10.347s), and `git diff --check` on the two owned files (pass). No PostgreSQL or live authenticated OpenCode path was run for this bounded re-review.
