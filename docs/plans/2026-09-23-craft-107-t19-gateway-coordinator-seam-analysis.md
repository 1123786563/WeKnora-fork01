# T19 Gateway coordinator seam analysis

**Status:** Read-only analysis for root ownership amendment. No production or test source was changed.

**Worktree / HEAD:** `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`, `a5e9195acd6500c085c85d60c852148e7bbbbf34`.

**Inputs:** `2026-09-23-craft-107-t19-gateway-body-fix-review.md`, `2026-09-23-craft-107-t19-gateway-high-fix-design.md`, `internal/application/service/craft_budget.go`, `internal/handler/craft_model_gateway.go`, and the existing gateway/start tests. The independent re-review reproduced the already recorded scoped test commands; no new command was needed for this analysis.

## Findings from the current flow

`CraftBudgetService.StartBinding` commits the call row, dispatched G4 reservation and `state='intent'` journal, then invokes the callback with a 30 second context. It persists the callback result with an `intent` compare-and-swap before returning. The gateway callback ends when `Do` returns response headers, so the service changes the journal to `started` before body read, size validation or close. Any later body failure records unknown usage but cannot undo that durable resolution. Repository lifecycle code regards only `intent` and `unknown` as unresolved, so a concurrent or later Run transition can release its fence after body failure.

The previous body-lifetime correction bound `Do` to `responseCtx` (default five minutes) and only checked `startCtx.Err()` once before calling it. That keeps delayed body bytes readable, but a `Do` blocked waiting for headers is not canceled at the coordinator's 30 second initiation deadline.

One standard `Request.Context` cannot independently impose an immutable 30 second timeout for `Do` and a longer timeout for `Response.Body`: Go's HTTP transport carries that same request context through response-body reads. Reading the whole body in the existing `StartBinding` callback is also invalid because the callback's bounded context expires at 30 seconds and `StartBinding` forces any late non-definite result to `unknown`.

## Proposed minimum durable coordinator seam

Add a two-phase start attempt in `internal/application/service/craft_budget.go` while retaining the existing `StartBinding` contract for current sandbox/other callers:

1. `BeginBinding` performs the existing transactional prepare unchanged: charge call, dispatched hold and unique journal `intent` commit together. On denial it returns before transport, as today. On success it returns an opaque attempt handle with a coordinator-owned 30 second initiation context. The context should not expose mutable journal identifiers.
2. `Resolve(outcome)` on that handle performs a tenant/run/activity-key scoped CAS from `intent` to exactly one of `started`, `unknown` or `definitely_unstarted`. It returns conflict if another resolver already won. Use a bounded persistence context that survives inbound request cancellation; if persistence fails, `intent` and the dispatched hold remain unresolved. The existing schema already supports every state; no migration or repository method is needed.
3. Keep `intent` durable while the gateway waits for headers and the complete bounded body. A body failure resolves to `unknown` (still unresolved). A complete bounded read plus successful `Close` resolves to `started`. Only a pre-`Do` cancellation can be definitely unstarted; once `Do` is called, transport and response errors are ambiguous. `started` must not be committed merely on headers.

An implementation can express the handle as a service interface with private concrete fields, for example `InitiationContext() context.Context`, `Resolve(outcome) error`, and `CancelInitiation()`. The handler must only receive this handle from the production `CraftBudgetService`; service tests should verify it cannot change a non-`intent` row and that a result-write error leaves the hold/journal fenced. `StartBinding` can continue using the existing prepare/resolve internals and its current one-phase semantics for its narrower callback users.

## HTTP context lifetimes with the new handle

Create `responseCtx` from the inbound request with the existing whole-response `g.timeout`. Make the outbound request context a cancelable child of `responseCtx`. Register `context.AfterFunc(attempt.InitiationContext(), ...)` whose callback cancels the outbound request only if `Do` has not yet returned headers. Protect the `Do`-returned flag with a mutex so the timeout/return race is conservative:

- If initiation context expires before headers return, cancel the request so a blocked `Do` ends. Since `Do` was invoked, resolve `unknown`, close any nonnil response body and record unknown usage.
- If headers return first, mark that phase complete and stop the initiation callback before canceling the attempt's initiation context. Keep the outbound request context alive under `responseCtx` through body read and `Close`.
- If inbound cancellation or the whole-response timeout occurs while reading, the transport body is canceled; resolve `unknown`, keep the G4 hold and journal unresolved, and return an error.
- Run lifecycle operations racing body completion remain safe: while body result is unknown/unavailable, `intent` holds the Run in reconciliation. If cancellation commits first, a later `started` result remains behind its durable cancel request and recovery resolves to canceled. If successful `started` commits first, later cancellation observes the resolved effect and can terminate the Run. No lifecycle path can observe `started` until complete body success is established.

## Concrete test and file ownership proposal

Proposed write ownership needs parent amendment before source edits:

- `internal/application/service/craft_budget.go`: extract/reuse start-intent prepare and CAS resolution, add the opaque two-phase handle, preserve existing StartBinding behavior.
- `internal/application/service/craft_budget_start_test.go`: real SQLite service tests for pre-callback committed intent/hold, `intent -> unknown` body ambiguity, `intent -> started` only after full observation, one-winner CAS/replay refusal, cancellation while unresolved, and injected result-write failure retaining intent/hold.
- `internal/handler/craft_model_gateway.go`: use the handle, split initiation/header and body lifetimes, classify errors and defer started resolution until body read/close succeeds.
- `internal/handler/craft_model_gateway_test.go`: real `httptest` upstream coverage for blocked headers canceled by a short initiation context, headers-first delayed body surviving the initiation deadline, body read/close/oversize unknown outcome, `(response,error)` close, and same-key no second send. A narrow fake handle can test handler status mapping, but not replace service-backed journal evidence.

No `internal/application/repository` source, container assembly, migration, or activity-identity/runtime files need changing for these two findings. `CraftModelGateway` receives the concrete service through its current `CraftChargeStarter` dependency slot; only that interface contract and handler test doubles need the new method shape.

## Scope boundary

This seam closes the response-result journal race and initiation timeout only. It does not close the separate High activity-identity finding: OpenCode still needs a durable server-owned per-provider-attempt ID/proof that survives retry/restart. Keep that integration gate explicit.
