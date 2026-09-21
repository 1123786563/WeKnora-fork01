# A03 Task 3 report

## Files

- `internal/application/service/semantic_model.go`: consumer-owned Go model gateway.
- `internal/application/service/semantic_model_budget.go`: pre-dispatch platform hold adapter and BYOK usage observation path.
- `internal/application/service/semantic_model_test.go`: capability denial and completed replay ordering regressions.
- `internal/application/repository/semantic_model_invocation.go`: durable `claimed -> dispatched` transition and restricted terminal transitions.
- `internal/types/interfaces/semantic_model.go`: `MarkDispatched` store contract.
- `migrations/sqlite/000101_semantic_model_invocations.up.sql` and `migrations/versioned/000180_semantic_model_invocations.up.sql`: add `dispatched` ledger state.
- `internal/container/container.go`: fail-closed gateway assembly while immutable production rates are unavailable.

## TDD evidence

RED command specified by the brief (before gateway implementation):

```text
go test ./internal/application/service ./internal/application/service/commercial -run '^TestSemanticModel' -count=1
```

The baseline had no Task 3 gateway/tests, so the command completed without exercising a semantic gateway. The focused gateway/repository regressions were then added and run after implementation:

```text
go test ./internal/application/service ./internal/application/service/commercial ./internal/application/repository -run '^TestSemanticModel' -count=1
ok github.com/Tencent/WeKnora/internal/application/service
ok github.com/Tencent/WeKnora/internal/application/service/commercial [no tests to run]
ok github.com/Tencent/WeKnora/internal/application/repository
```

## Verification

```text
git diff --check
go test ./internal/application/service ./internal/application/service/commercial ./internal/application/repository -run '^TestSemanticModel' -count=1
go test ./internal/container -run '^$'
```

All commands exited 0. The container package emitted only the existing linker warning about duplicate `-lc++` libraries.

## Migration / integration caveat

The state check was changed in the Task 2 creation migrations because this branch has not introduced a separate forward migration for already-deployed databases. A deployed database needs a forward migration that expands the invocation-state constraint before this code is rolled out.

The registered production gateway intentionally has no immutable rate resolver and therefore fails closed during capability verification / budget admission. Pricing administration must supply one shared immutable resolver to enable real model dispatch.

## Self-review and concerns

The dispatch ordering retains the platform hold and task quota after either dispatch marker is ambiguous; provider I/O follows both markers only. Model resolution occurs after platform reserve; completed replay exits before model resolution. BYOK has no platform reservation and records its owner-tenant raw usage through `UsageStore` when a real resolver/store is injected.

Concern: provider `ChatResponse` exposes integer token fields without a presence bit. The gateway intentionally treats zero prompt or completion usage as unknown to avoid inventing a zero settlement; this is conservative but providers that legitimately report a zero completion will require a richer provider-usage contract to settle it.

## Review fix round 1

Files changed in this round:

- `internal/application/service/semantic_model.go`
- `internal/application/service/semantic_model_budget.go`
- `internal/application/service/semantic_model_test.go`

The gateway now rejects elapsed/zero capability deadlines before scope/model work, places owner scope/model resolution, dispatch markers, and provider I/O beneath that deadline, and persists post-dispatch unknown state through an independent five-second owner-tenant cleanup context. Provider usage now requires positive prompt, completion, and total fields; total must exactly equal overflow-safe prompt plus completion and each component must stay under its capability cap. Invalid facts remain unknown and do not settle.

BYOK admission now requires a non-empty exact immutable rate version, a resolver returning that version, and an owner raw-usage store before it can resolve a model. It still never reserves or settles platform Credits. Focused counted seams in the budget adapter drive gateway-only ordering tests without modifying generic `ExecutionGate.Begin` or Craft APIs. `semantic_model_test.go` covers rates admission, deadline denial/cancellation, platform denial, pre-dispatch model-resolution release, dispatch-marker ambiguity, malformed usage retention, successful one-time settlement, and completed replay.

Observed verification:

```text
$ go test ./internal/application/service ./internal/application/service/commercial ./internal/application/repository -run '^TestSemanticModel' -count=1
ok   github.com/Tencent/WeKnora/internal/application/service  1.660s
ok   github.com/Tencent/WeKnora/internal/application/service/commercial  1.308s [no tests to run]
ok   github.com/Tencent/WeKnora/internal/application/repository  6.188s

$ go test ./internal/container -run '^$'
# github.com/Tencent/WeKnora/internal/container.test
ld: warning: ignoring duplicate libraries: '-lc++'
ok   github.com/Tencent/WeKnora/internal/container  (cached) [no tests to run]

$ git diff --check
(no output; exit 0)
```

## Review fix round 2

The commercial repository now has the intentionally narrow
`UsageStore.RecordRawModelUsage(ctx, fact)` path. It accepts only a final
BYOK `ServiceModel` fact containing the model dimension, writes its physical
owner usage row and current-revision pointer with `ChargeMicro=0`, and creates
no `usage_settlement` outbox record. Generic `UsageStore.Record` retains its
existing billable-final behavior. `SemanticModelBudgetAdapter.finish` uses the
new raw path for BYOK and continues using `ExecutionGate.Finish` for platform
funding.

Added/strengthened behavioral tests:

- `TestSemanticModelRecordRawBYOKFinalPersistsWithoutSettlement` exercises
  the real SQLite `UsageStore` and verifies owner persistence, current pointer,
  zero charge, and no settlement outbox event.
- `TestSemanticModelBYOKPersistsOwnerRawUsageWithoutPlatformSettlement` runs
  the gateway with the real raw usage store and verifies owner/funding/price
  binding, one claim/model/provider/completion path, no platform gate, and no
  settlement event.
- `TestSemanticModelOwnerAdmissionDenialsPrecedeModelResolution`,
  `TestSemanticModelBudgetDispatchMarkerFailureRetainsHoldAndSkipsProvider`,
  and `TestSemanticModelMalformedUsageBecomesUnknownWithoutSettlement` cover
  ACL/quota admission ordering, second-marker ambiguity, and end-to-end
  malformed provider usage. The completed-replay and platform-success tests
  now count and assert no repeat reserve/provider/finish plus exact owner
  reservation, funding, and immutable rate version respectively.

TDD evidence: the new repository test initially failed to compile because
`RecordRawModelUsage` did not exist. The new gateway BYOK test then failed with
one `usage_settlement` event while `finish` still called generic `Record`.
After the narrow method and adapter routing were added, both tests passed.

Fresh verification output:

```text
$ go test ./internal/application/service ./internal/application/service/commercial ./internal/application/repository -run '^TestSemanticModel' -count=1
ok   github.com/Tencent/WeKnora/internal/application/service  2.713s
ok   github.com/Tencent/WeKnora/internal/application/service/commercial  2.052s [no tests to run]
ok   github.com/Tencent/WeKnora/internal/application/repository  5.094s

$ go test ./internal/application/repository/commercial -run '^TestSemanticModel' -count=1
ok   github.com/Tencent/WeKnora/internal/application/repository/commercial  0.441s

$ go test ./internal/container -run '^$'
# github.com/Tencent/WeKnora/internal/container.test
ld: warning: ignoring duplicate libraries: '-lc++'
ok   github.com/Tencent/WeKnora/internal/container  (cached) [no tests to run]

$ git diff --check
(no output; exit 0)
```
