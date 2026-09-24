# T19 model gateway response lifetime fix report

**Status:** High finding 1 is corrected at the handler seam. High finding 2 (runtime attempt identity propagation/validation) remains a separate integration blocker; this report does not claim T19 Task 3 end-to-end completion.

**Worktree / baseline:** `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`, HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. No commit or staging. Other shared-worktree changes were preserved.

## Implementation

- `Forward` now creates a whole-response context from the inbound request with the existing `g.timeout` bound. `StartBinding` still owns the bounded initiation callback; the callback checks its context immediately before `Do`, but does not cancel the transport request context when response headers return. The response context remains active through body consumption and `Close`, preserving caller cancellation and the whole-response timeout.
- Body read uses a `craftMaxForwardBody+1` limit to detect truncation/oversize. Read errors, oversize bodies, nil bodies, and body close errors record unknown usage and return 502. They do not turn the activity into a replayable operation.
- Any non-success result after a transport response is returned closes its body, including `(response, error)` and `StartBinding` error paths.

## TDD and verification

RED was first attempted but Go package compilation was blocked by a concurrent unowned edit: `internal/application/service/craft_session.go:854` referenced undefined `craftActorInputAdmissionRequestID`. This task did not modify that file. Once the shared source compiled, the new delayed-body test and focused suite were run against the implementation; the test proves the callback has returned while the response request context remains active and body bytes are still pending.

Commands and results on the final source snapshot:

```text
gofmt -w internal/handler/craft_model_gateway.go internal/handler/craft_model_gateway_test.go
PASS

go test ./internal/handler -run 'TestCraftGateway|TestCraftModelActivityKey' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/handler 0.703s

go test -race ./internal/handler -run 'TestCraftGateway|TestCraftModelActivityKey' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/handler 2.337s

git diff --check -- internal/handler/craft_model_gateway.go internal/handler/craft_model_gateway_test.go
PASS
```

Test additions exercise delayed body availability after `Do` returns, context liveness until body release, successful body close, body-read failure non-replayability/unknown usage, and response-plus-transport-error body close/unknown usage. The existing same-activity replay and transport uncertainty checks also pass.

## Checkpoint

The exact full-content checkpoint is recorded in `2026-09-23-craft-107-t19-gateway-body-fix-checkpoint.json`, with the owned source/test delta from the recorded HEAD in `2026-09-23-craft-107-t19-gateway-body-fix-checkpoint.patch`. Checkpoint SHA-256 values:

```text
internal/handler/craft_model_gateway.go
c2d754451fbed3d9e9aace619665892ecebc7e228c946f2eed96d0ce6dcc47d7
internal/handler/craft_model_gateway_test.go
a13c6ece2f9eec8dca350dfc3ead1447c7ec8d0e5d8dc88f273320a895af5e4f
```

The patch is relative to the original Task 3 baseline and therefore also contains that already independently reviewed gateway implementation. This checkpoint is limited to the two owned handler files; no container/runtime/budget files were edited.

## Remaining limits

- Runtime-generated durable attempt identity is not propagated through the real OpenCode-to-provider request path. Gateway High finding 2 remains open and still blocks production-use/Task 3 completion.
- Focused handler package tests passed; no database, PostgreSQL, live provider or pinned OpenCode path was changed or verified by this fix.
