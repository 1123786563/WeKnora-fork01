# T19 Task 3 — Model gateway initiation checkpoint

**Status:** The gateway `Client.Do` path now requires a stable, credential-scoped activity ID and invokes the typed durable start coordinator immediately around bounded transport initiation. The coordinator commits intent/G4 hold before callback by contract. Tests pass against a fake coordinator and existing SQLite service tests. Runtime propagation of the activity header is not integrated, so this is not end-to-end model-call verification and current OpenCode callers without the header fail closed before sending.

**Role:** `backend_implementer`. Model and reasoning-effort metadata were not exposed; none is asserted.

**Worktree / HEAD:** `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`; HEAD before this task `a5e9195acd6500c085c85d60c852148e7bbbbf34`. No commit or staging. Shared worktree changes were preserved.

## Implementation

- Replaced the handler's `AuthorizeBinding` send authority with `CraftChargeStarter.StartBinding`. Constructor requires a budget port, typed starter, recorder, and upstream resolver; missing starter fails closed.
- Production container assembly passes the same `CraftBudgetService` as BudgetPort and Starter. The service owns `StartBinding`, which commits journal intent and dispatched G4 hold before invoking its bounded callback; the handler does not open a transaction or call the provider before that callback.
- Requires `X-Craft-Activity-ID` for a forward. The value is validated (1–128 ASCII token characters) and hashed with authenticated tenant/run/grant scope. Binding facets stay outside the key, so reusing the key with a changed model/delegation/funding reaches the same durable key and is refused by `StartBinding`. The raw value is not journaled. Re-issued credentials for the same grant preserve the activity key.
- Upstream target lookup and request construction happen before the durable start, so missing credentials/invalid request inputs do not reserve or send. `Client.Do` is called only from the start callback with a bounded context. Transport errors after entering `Do` return `Unknown`; an error after possible send is never retried. The response body and usage recording remain outside SQL. A body-read failure records unknown usage and the same activity key remains non-replayable.
- Same-key/restart refusal is represented as HTTP 409 `ACTIVITY_UNRESOLVED`. Missing activity identity is HTTP 400; there is no fallback `AuthorizeBinding` send path in the handler.

## TDD and verification

RED was observed before the gateway changes:

```text
go test ./internal/handler -run 'TestCraftGateway(RequiresStableActivityIdentityBeforeAnyUpstreamWork|SameActivityKeyCannotStartTwice)$' -count=1
FAIL: missing-identity case expected 400, got 200; same-key replay expected 409, got 200
```

GREEN and current focused results:

```text
go test ./internal/handler -run 'TestCraftGateway|TestCraftModelActivityKey' -count=1
ok   github.com/Tencent/WeKnora/internal/handler  1.105s

go test -race ./internal/handler -run 'TestCraftGateway|TestCraftModelActivityKey' -count=1
ok   github.com/Tencent/WeKnora/internal/handler  2.426s

go test ./internal/container -run 'CraftModelGateway|CraftUpstreamResolver' -count=1
ok   github.com/Tencent/WeKnora/internal/container  4.683s

go test ./internal/application/service -run '^TestCraftChargeStart' -count=1
ok   github.com/Tencent/WeKnora/internal/application/service  12.896s
```

Container tests emitted the linker warning `ignoring duplicate libraries: '-lc++'`; they passed. The service run is existing Task 1 SQLite evidence that the actual coordinator makes intent/hold visible before its callback, keeps unknown outcomes/holds, and refuses same-key callback replay. `git diff --check` and `gofmt` passed on the owned source/test files.

The focused handler tests cover required/malformed identity, stable run/grant binding across credential reissue, same-key replay including changed model, visible budget denial, post-send transport error without replay, body-read error without replay, unknown usage recording, managed upstream credential use, and no activity intent when upstream resolution fails. The constructor test proves a gateway cannot be assembled without Starter. Container tests cover the existing assembly path and fail-closed upstream/secret configuration.

## Integration limits and unresolved risks

- I searched the repository for `X-Craft-Activity-ID` and model-gateway clients. This header exists only in this handler/test delta; the current OpenCode runtime caller is not assembled in the existing model-gateway container code (the assembly comment says it remains gated until a runtime caller exists). Therefore the runtime must propagate the same activity ID across retries before real model traffic can use this endpoint. Until then, requests lacking it receive 400 before upstream lookup, hold reservation, or `Client.Do`. No runtime file was edited under this Task's ownership.
- The request key is supplied by the authenticated runtime request and bound to the signed execution run/grant scope. Correct retry identity depends on that runtime preserving the same header value. This task does not create an upstream idempotency guarantee; unresolved activity IDs remain non-replayable.
- A `Client.Do` error is recorded as unknown usage and journal outcome `unknown` through Task 1. When `Do` returns a response but subsequent body reading fails, the journal's initiation outcome remains `started` (the transport returned response headers); usage is recorded unknown and duplicate sends remain blocked. Changing that journal state after body read needs an outcome-update seam outside this file ownership.
- The named `docs/plans/2026-09-23-craft-107-t19-worker-seam-map.md` was not present in this worktree. I read the durable-start Task 3 and the available `2026-09-23-craft-107-t19-budget-pause-seam-design.md`; the current gateway source directly confirmed the unfenced send path.
- No PostgreSQL race test, live authenticated OpenCode call, or full Task 3 production caller test ran. T19 Task 4 sandbox initiation and the separately assigned budget pause writers remain outstanding; this checkpoint does not close F3/T19.

## Owned source SHA-256

```text
internal/handler/craft_model_gateway.go
d9f5946acf1ac7b8bf378f18f778fcf095e8e7e2a50b473dedec132451fd170e
internal/handler/craft_model_gateway_test.go
8a5bb465bcb61089e5ff2eefa488ad5cb1442107848c515a47b543e98665adbb
internal/container/craft_model_gateway.go
2dd40aae829b41b02445fa20339ff279d57d99897f03f19ab64edbed8f9d0b0c
```
