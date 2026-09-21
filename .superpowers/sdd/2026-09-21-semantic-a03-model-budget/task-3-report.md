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
