# P0 gate decision — tRPC native Agent migration

Decision date: 2026-09-19. Evidence baseline: `950716d62f0a7ff1b06865dcf1f025bc434d7873`. This is a gate decision, not product acceptance or a production-switch authorization.

## Decision

**NO-GO for native Runner product execution.** The historical v1.10.0 baseline fails the repeated SDK Session race gate; the current exact v1.11.0 root candidate passes the bounded native Runner normal probe and `-race -count=20` gate. This only resolves that candidate's reproduced race. Current `internal/application/service` fails `TestExecuteDurableRunPersistsBudgetExhaustionForNotification`, and the exact `75523c9c9` v1.10.0 baseline fails the same test with the same missing notification event, so it is an existing direct-consumer blocker rather than a v1.11.0 regression. No Session/Memory persistence configuration has been selected. Product code must therefore not construct or dispatch a native Runner, including through MCP, Skills, child Agents, or a feature flag.

This does not erase the independent P1/P2 work still required by the confirmed specification. It prevents those work packages from using a draft `nativecontract` or an unapproved SDK/storage composition as though it were an executable product interface. No reduction of recovery, authorization, tenant isolation, archive retention, or existing functionality is approved.

## Evidence matrix

| Classification | P0 evidence | Decision impact |
| --- | --- | --- |
| Measured | Task 2's deterministic `LLMAgent`/Runner/function-tool round trip completed exactly one tool call and a final `finished` message. Its in-memory Session key test kept two distinct `AppName` values apart. Task 3's SQLite checkpoint/pending-write and recovery-policy suites passed. | These bounded probes do not prove authorization, production persistence, real Provider behavior, cross-process recovery, or release readiness. |
| Source-only | Historical v1.10.0 source and current v1.11.0 candidate compilation define Runner, callbacks, Session/Memory APIs, Graph checkpoints and the interfaces captured in `interfaces.md`. The capability matrix has 79 source-only rows. | Source reading and bounded compilation fix neither persistence behavior nor an implementation selection. |
| Blocked environment | `TestCrashMatrixPostgreSQL` and `TestTwoWorkerContentionPostgreSQL` require `TRPC_RECOVERY_PG_DSN`; `TestCrashAfterToolResult` requires `TRPC_RECOVERY_GRAPH_PROVIDER`. All three were explicitly skipped. | PostgreSQL and real-Provider recovery remain `blocked-env`, never passed by SQLite or deterministic models. |
| Behavior incompatible | Historical `RUN-02` records the v1.10.0 `Session.Clone` / `UpdateUserSession` race; current v1.11.0 repeated race evidence is green. `SESSION-05` does not make an append failure a durable barrier. `EVENT-01` preserves the old warning-only append gap. `SKILL-02` can fall back to a local executor. `MODEL-06` shows the old bridge rejects required fields. | The current candidate's race green is necessary only. The consumer budget-notification failure and all remaining rows retain required P1/P2/P3/P4/P5/P7 remediation and acceptance. None is an authorization to bypass the affected boundary. |

The matrix totals at this baseline are 4 `verified`, 79 `source-only`, 2 `blocked-env`, and 5 `incompatible` rows. The root now pins `trpc-agent-go v1.11.0`; Task 1's bounded candidate evidence is recorded in `sdk-probes.md`, but it is **not** an approved product target. No Session backend, Memory backend, or complete root/submodule compatibility set has been selected.

## Required gate resolution and downstream ruling

Before any native Runner product execution, a separate reviewed task must approve the exact current `v1.11.0` SDK tag/commit, every Session/Memory storage submodule and configuration, and their checksums. The candidate has passed the fixed repeated race gate; a one-off pass would not be unlock evidence:

```sh
GOWORK=off go test -race ./internal/agent/nativeprobe -count=20 -v
```

All twenty executions exited zero for the current v1.11.0 candidate with no race report. That result is necessary but not sufficient. The selected composition must first repair the existing budget-exhaustion notification consumer failure, then establish the Session append-failure barrier, durable event/outbox behavior, six recovery transitions, tenant/principal isolation, SQLite and PostgreSQL persistence, and the required real-Provider matrix. Historical v1.10.0 remains a rejected RED baseline because the repeated gate reported the SDK race; the current candidate's green evidence does not turn it or the product path into an approved execution composition.

| Downstream work | Ruling now | Unlock evidence |
| --- | --- | --- |
| P1 storage / data authority | Do not implement or create an executable plan. The draft Session/Memory interfaces do not identify a selected persistent service, its concurrency behavior, or compatible version set. | Exact Session/Memory backend and root SDK selection; clean race probe; SQL dialect/backend capability probe; reviewed persistence API and transaction boundary. |
| P2 governance / tool boundary | Do not implement or create an executable plan. Its admission, journal, pending-decision and event contracts depend on P1's durable storage boundary, and it may not use a Runner to work around the gate. | P1's reviewed identity/error/logging/storage interface plus the native execution gate above. Until then, existing production governance remains authoritative. |
| P3/P7 recovery | Blocked for the native composition. Existing SQLite graph evidence remains baseline-only. | P1/P2 integration plus SQLite/PostgreSQL process-kill matrices, real Provider query/idempotency evidence, and each of the six transition tests. |
| P4/P5 and release/cutover | Not unlocked. Skills cannot use implicit local execution; clients cannot receive warning-only event delivery as reliable state. | Their own approved P3/P7 evidence and all required provider/client/operational acceptance. |

Consequently, P1/P2 executable plans are intentionally not created in this commit: the two paths named by P0 Task 5 are `2026-09-19-trpc-native-agent-p1-storage.md` and `2026-09-19-trpc-native-agent-p2-governance.md`. Creating detailed RED/GREEN code plans would require inventing SDK Service calls and migration/storage semantics before their prerequisite evidence exists, which would falsely bypass this no-go. The future planning task must use the selected, verified API rather than this document's draft contracts.

## Migration and deletion boundary

At HEAD `950716d62f0a7ff1b06865dcf1f025bc434d7873`, the highest active migration files are `migrations/versioned/000158` and `migrations/sqlite/000079`; `migrations/mysql/` contains only `00-init-db.sql`. No migration number is reserved or allocated here because neither P1 plan is executable. The responsible future storage task must reread the active migration heads after serial integration and allocate its numbers then; it must provide matching up/down SQL only for active dialects selected by the reviewed backend decision.

No old execution consumer may be removed at P1/P2. The deletion boundary remains the Task 1 feature inventory and the confirmed specification: existing model consumers outside Agent execution, current production Agent routes, client protocols, business Run/ToolCall/UserDecision records, approval/OAuth paths, audit records, attachments, and archive readers remain until their replacement has passed its own scoped and integration acceptance. Old sessions are read-only archive data; they are neither deleted nor imported into a new Session.

## Review-focus coverage retained for later plans

| Review focus | Responsible work | Required test evidence before acceptance |
| --- | --- | --- |
| Same user/session strings in two spaces | P1, P3, P6 | SQLite and PostgreSQL conflicting-scope create/read/list/reopen tests; archive authorization test. |
| Event channel ends before task completes | P3, P5, P7 | Final-event/terminal-state fault test and durable outbox/replay test; no final SDK event may imply completion alone. |
| Tool success before result persistence | P2, P3, P7 | Kill after external success before result commit: query/idempotent reuse or `waiting_user`; non-queryable non-idempotent calls are never replayed. |
| Event append failure is swallowed | P1, P3, P5, P7 | Inject append failure and prove visible failure/recovery with zero subsequent dispatch; rebuild/replay the projection. |
| Real Provider missing while recovery tests skip | P3, P7 | Required real-Provider command with an authorized provider and PostgreSQL matrix; skipped environment is a release-blocking `blocked-env` result. |

P1/P2 share the scope, typed-error, admission, journal, event and checkpoint coordination contracts in `interfaces.md`. A future serial interface owner must review and freeze those contracts before parallel implementation; no concurrent task may independently alter `runtime/contracts.go`, container wiring, shared migrations, or common event types.

## Evidence record

This ruling consumes the committed Task 1–4 artifacts: `features.tsv`, `sdk-probes.md`, `recovery-gaps.md`, `sdk-capabilities.tsv`, and `interfaces.md`. It adds no SDK, production, migration, Provider, database, or client behavior evidence. P0 itself is a decision/planning gate; it does not mark the migration, P1/P2, or any release condition as passed.
