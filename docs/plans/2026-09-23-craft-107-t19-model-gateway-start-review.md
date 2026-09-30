# T19 Task 3 — independent model gateway start review

Date: 2026-09-23. Read-only review of the Task 3 owned gateway checkpoint against the approved #107/T19 acceptance snapshot, `CONTEXT.md`, ADR-0004, the durable-start plan Task 3, and the reviewed Task 1 denial-path fix. The integration worktree is shared; no source or test files were changed and OCR was not run.

## Checkpoint and verdict

The live owned files match the implementation report's SHA-256: `internal/handler/craft_model_gateway.go` `d9f5946acf1ac7b8bf378f18f778fcf095e8e7e2a50b473dedec132451fd170e`; `internal/handler/craft_model_gateway_test.go` `8a5bb465bcb61089e5ff2eefa488ad5cb1442107848c515a47b543e98665adbb`; `internal/container/craft_model_gateway.go` `2dd40aae829b41b02445fa20339ff279d57d99897f03f19ab64edbed8f9d0b0c`.

- **Scoped Spec compliance: FAIL.** The handler uses the typed `StartBinding` coordinator and production container passes the real budget service, so the commit-before-`Do` seam is present. However, the gateway has no runtime-propagated stable activity identity, and a valid forward with no new header is rejected. The endpoint is not currently usable for the production OpenCode model path. The response-body lifecycle also violates the Task 3 requirement to handle body and usage after initiation.
- **Scoped code quality: FAIL.** A bounded request context is canceled before the response body is consumed, which breaks streaming and delayed response bodies after a successfully initiated call. The resulting unknown usage and non-replayable activity make this more than a cosmetic HTTP error.
- **Full T19/#138: NOT VERIFIED.** Sandbox initiation, complete runtime assembly, PostgreSQL races, and joined acceptance remain outside this checkpoint.

## Findings

### 1. High — successful `Do` cancels the response body before reading it

**Evidence / symbol:** `CraftModelGateway.Forward`, `internal/handler/craft_model_gateway.go:503-516,537-544`. The callback creates a child request context and defers `cancel()`. It returns immediately after `g.forward(req.WithContext(ctx))` produces response headers. Both that defer and the coordinator's bounded-context defer (`internal/application/service/craft_budget.go:293-319`) cancel the HTTP request context before `Forward` calls `io.ReadAll(resp.Body)`. Go's HTTP transport uses the request context for the response body lifetime. The existing happy-path test sends a small, immediately available body; no test holds body bytes until after headers or streams them.

**Impact:** A normal delayed or streamed model response can fail at body read after the provider accepted the call. The gateway returns 502, records unknown usage, and the durable activity key blocks retry. Model output is lost although the chargeable call occurred.

**Smallest defensible correction:** Preserve a bounded response-body context through read and close while keeping only the `Do` initiation inside `StartBinding`; then add a real HTTP test where headers arrive first and body bytes arrive after the callback returns. Keep the journal/hold unresolved on any later ambiguous body failure. The response body also needs closing on `startErr` after `Do` produced a response (currently the early return at `:517-529` skips `Close`).

### 2. High — production caller cannot supply the required activity identity

**Evidence / symbol:** `CraftModelGateway.Forward`, `internal/handler/craft_model_gateway.go:474-478`; `newCraftModelGatewayHandler`, `internal/container/craft_model_gateway.go:98-140`; OpenCode client request construction, `internal/modules/agentruntime/agent/opencode/client.go:260-308`. The new header is referenced only by this handler and its tests. The runtime/client constructs HTTP requests without it, and the current container assembles an OpenCode client aimed at `CRAFT_OPENCODE_BASE_URL`, with no activity-ID propagation to model calls. The implementation report independently acknowledges this missing integration.

**Impact:** Existing runtime model requests fail with 400 before G4 reservation or upstream send when directed through the gateway. Without a durable, retry-stable ID from the authenticated runtime, the handler also cannot prove that a retry with a new header represents the same physical activity; its hash authenticates scope but does not authenticate the chosen activity value.

**Smallest defensible correction:** Define and propagate a server-owned model-attempt ID from the runtime's durable attempt record through the actual OpenCode model request, preserving it across retries/restarts; bind and verify it at the gateway. Add a production-path test of first send, same-attempt retry and new-attempt distinction. If the pinned runtime cannot propagate such an ID, keep the gateway fail-closed and explicitly block Task 3 completion.

## Positive evidence and limits

The handler now calls `StartBinding` around `Client.Do`, and production assembly supplies `CraftBudgetService` for both budget and starter. Activity keys are bound to signed tenant/run/grant scope and ignore mutable model/delegation/funding facets, so same-header binding changes reach the same journal uniqueness check. The fake starter tests verify same-key rejection and transport-error non-replay; the reviewed Task 1 service implementation commits intent and dispatched G4 hold before its callback. The handler records unknown usage after transport or body-read error. No `AuthorizeBinding` forward path remains in the owned handler.

I ran `go test ./internal/handler -run 'TestCraftGateway|TestCraftModelActivityKey' -count=1` (pass, 0.724s), `go test ./internal/container -run 'CraftModelGateway|CraftUpstreamResolver' -count=1` (pass, 2.087s; linker warned about duplicate `-lc++`), and `git diff --check` on the three owned files (pass). These tests do not exercise a delayed HTTP response body, a real runtime model request, or PostgreSQL coordination. A live authenticated call and full T19 acceptance therefore remain unproven.
