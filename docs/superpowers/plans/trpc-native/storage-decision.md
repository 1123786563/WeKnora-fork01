# P1.0 storage-decision — tRPC Native Agent

Decision date: 2026-09-19.  Base examined: `f5caa23eef892b3b33d8e1b62398152868fd95a5`.
This is an evidence and contract decision.  It neither enables the native
Runner nor authorizes a production migration, new wiring, or a change to the
P0 product-execution ruling.

## Decision

**P1.0 approves and freezes the WeKnora-controlled Session/Memory facade plus
business-database adapter as the P1 implementation boundary.**  It unlocks
P1.1, then P1.2 after P1.1 meets its own DAG gate.  No raw tRPC Session or
Memory SQL backend is selected for product use.  This boundary approval does
not enable native Runner product execution: that remains P0 `NO-GO`.  P1.3,
P1.4, P1.5 and P2 proceed only in their DAG order with their own RED/GREEN,
review, and integration evidence; P7 remains the integrated acceptance gate.

| Candidate | Decision | Evidence and consequence |
| --- | --- | --- |
| Root `trpc-agent-go v1.11.0` | retained SDK API candidate; **not** an approved product composition | The independent probes compile against it and their default suites pass, but the root only has a bounded repeated Runner race green.  Direct-consumer budget notification remains failing and the storage contract is not met. |
| `session/sqlite v1.11.0` | rejected as a raw product backend | Replaying a stable `event.Event.ID` with changed payload returned nil in the forced contract test.  Duplicate stable IDs are stored; an append can mutate the caller object before persistence fails. |
| `session/postgres v1.11.0` | rejected as a raw product backend | Historical isolated-DSN characterization has the same stable-ID conflict failure and default schema initialization fails its unique-index check.  This run had no DSN, so PostgreSQL re-verification is `blocked-env`. |
| `memory/sqlite v1.11.0` | rejected as a raw product backend | A clear followed by old-generation extraction output returns nil in the forced contract test; no generation/tombstone/CAS input exists on raw `AddMemory`. |
| `memory/postgres v1.11.0` | rejected as a raw product backend | Historical isolated-DSN characterization has the same stale-write behavior.  This run had no DSN, so PostgreSQL re-verification is `blocked-env`. |
| WeKnora controlled Session/Memory facade and repository | selected target boundary, **implemented with acceptance gaps under review** | The integrated facade enforces authoritative scope, receipt/hash uniqueness, generation/tombstone CAS, event-bound memory jobs, transaction coordination, and authorization rechecks; remaining SDK tool/metadata coverage and live PostgreSQL evidence are tracked in the P1.3/P1.4 review ledger. |

Source reading, a mock/deterministic model, a skipped test, or a passing SQLite
unit test is not a pass for this decision.

## Fixed module composition observed from each nested module

All probes are standalone Go modules, ran with `GOWORK=off`, and have no local
`replace` directive.  The Go checksum is the module zip sum, followed by its
`go.mod` sum.

| Use | Exact module | Version | zip sum | go.mod sum |
| --- | --- | --- | --- | --- |
| all three probes | `trpc.group/trpc-go/trpc-agent-go` | `v1.11.0` | `h1:LwMxQwT2l6hqWUVARfVA/ef2tq8gJCzbImSHurpaPIo=` | `h1:bIZcN4N9sGpA42sWfE98XCPl9ZgMi6fMDLGYOaXNe9A=` |
| Session probe | `trpc.group/trpc-go/trpc-agent-go/session/sqlite` | `v1.11.0` | `h1:LI5KVBWuG4JBt7advdO6osb+DmyqA9WvH8bxJ2E6uW8=` | `h1:CQOZ2lSwpwkYnN0hOWZLIOb4NMIhYZMd9n8gScfTnyw=` |
| Session probe | `trpc.group/trpc-go/trpc-agent-go/session/postgres` | `v1.11.0` | `h1:k6a6Sa9i2mjlLVZVmRnYwEFLdQeOsbIXb2aRFCVHxlg=` | `h1:f8iE70qCVVnXE/ECpMzSbtYwLcwE7oFGrXK37wit/M4=` |
| Memory probe | `trpc.group/trpc-go/trpc-agent-go/memory/sqlite` | `v1.11.0` | `h1:Xb5/BLdIhS0FPEdqDdtq5Mm3I3wQaytFYiGq+F24ln8=` | `h1:0DTbqSROnJ5JDSBLHy+b+EeXcB46BnjDkL3i89/DtYA=` |
| Memory probe | `trpc.group/trpc-go/trpc-agent-go/memory/postgres` | `v1.11.0` | `h1:l5osN13fRX8oj3luKydVbxd/s+VKXwDd5RN1DkT/w2g=` | `h1:TQFM2o53kVU0lQDqTX2GRB+wz1HV74CnOp62oIgFUL4=` |
| Memory probe | `trpc.group/trpc-go/trpc-agent-go/storage/postgres` | `v1.11.0` | `h1:t7sovgCKIVbxNETujKGvJ4hWZlQY+hoFmE2dNeT9piY=` | `h1:X/XSYESqsU2l80yTyi/ggBvD3ofQGyjqJU5NeSKe2/o=` |

The Session probe's indirect `storage/postgres v0.8.0` differs from the
Memory probe's explicitly resolved `v1.11.0`.  `v0.8.0` has zip sum
`h1:8phxJucSgwhdhS0UP6uyaVtWyNL3z1RW7OfLHYUtzco=` and go.mod sum
`h1:q1/qPdLQ8hgyGhvq2t0lmfhRcekm0chBc5+ZI32pBv0=`.  It is not selected for
the product composition.  That mismatch is a reason no combined raw SDK SQL
composition is approved; P1.2 pins its complete adapter graph.

## v1.11.0 SDK service surface (source-confirmed only)

These are actual interfaces in root package
`trpc.group/trpc-go/trpc-agent-go@v1.11.0`: `session/session.go` and
`memory/memory.go`.  They describe an SDK seam only.  The raw SQL backends
above remain rejected, and the listed methods do not provide the controlled
facade's authorization, receipt/hash, generation/tombstone, fence, or barrier
semantics.

| Service | Source-confirmed methods | P1.0 interpretation |
| --- | --- | --- |
| `session.Service` | `CreateSession(ctx, Key, StateMap, ...Option) (*Session, error)`; `GetSession(ctx, Key, ...Option) (*Session, error)`; `ListSessions(ctx, UserKey, ...Option) ([]*Session, error)`; `DeleteSession(ctx, Key, ...Option) error`; `UpdateAppState`, `DeleteAppState`, `ListAppStates`; `UpdateUserState`, `ListUserStates`, `DeleteUserState`; `UpdateSessionState`; `AppendEvent(ctx, *Session, *event.Event, ...Option) error`; `CreateSessionSummary`; `EnqueueSummaryJob`; `GetSessionSummaryText`; `Close() error`. | Source-confirmed complete root interface.  P1.3's controlled facade preserves the complete CRUD/state/summary method surface and adds business `AppendStable`-style authorization/idempotency extensions; raw backend implementations are not accepted merely because they satisfy this interface. |
| `memory.Service` | `ReadMemories(ctx, UserKey, int) ([]*Entry, error)`; `SearchMemories(ctx, UserKey, string, ...SearchOption) ([]*Entry, error)`; `AddMemory(ctx, UserKey, string, []string, ...AddOption) error`; `UpdateMemory(ctx, Key, string, []string, ...UpdateOption) error`; `DeleteMemory(ctx, Key) error`; `ClearMemories(ctx, UserKey) error`; `Tools() []tool.Tool`; `EnqueueAutoMemoryJob(ctx, *session.Session) error`; `Close() error`. | Source-confirmed root interface.  P1.4's facade must add scope recheck and generation/tombstone CAS around every delayed write; no raw Memory backend or auto-job method is selected for direct product use. |

## Active dialect ruling

P1 has two active business-database dialects: PostgreSQL for the normal
deployment and SQLite for Lite/test coverage.  Both require matching P1
migrations and behavior evidence.  SQLite is not a substitute for PostgreSQL
concurrency, recovery, or provider evidence.  Repository code contains some
MySQL compatibility branches, but the current migration layout has only the
PostgreSQL versioned set and SQLite twins for the relevant durable records;
there is no P1 MySQL backend, migration pair, or acceptance evidence.  MySQL
and every other dialect are therefore inactive for P1 and must stay unsupported
until a later reviewed decision adds the full schema and test matrix.

## Evidence re-run

The following commands were executed on this worktree.  The default suites
characterize local behavior; they do not override the deliberately forced RED
contract cases.

| cwd | command | exit | result |
| --- | --- | ---: | --- |
| `tools/trpc-native-p1/sessionprobe` | `GOWORK=off go test ./... -count=1` | 0 | SQLite characterization passed; PostgreSQL subtests skipped because `P1_SESSION_PG_DSN` is unset. |
| same | `GOWORK=off go test -race ./... -count=20` | 0 | no race reported; PostgreSQL still skipped. |
| same | `GOWORK=off go vet ./...` | 0 | no findings. |
| same | `P1_REQUIRE_CONTRACT=1 GOWORK=off go test -run TestSessionContractReplayRejection -count=1 -v ./...` | 1 | SQLite changed-payload replay with stable Event.ID returned nil; PostgreSQL skipped as `blocked-env`. |
| `tools/trpc-native-p1/memoryprobe` | `GOWORK=off go test ./... -count=1` | 0 | SQLite characterization passed; PostgreSQL subtests skipped because `P1_MEMORY_PG_DSN` is unset. |
| same | `GOWORK=off go test -race ./... -count=20` | 0 | no race reported; PostgreSQL still skipped. |
| same | `GOWORK=off go vet ./...` | 0 | no findings. |
| same | `P1_REQUIRE_CONTRACT=1 GOWORK=off go test -run TestMemoryContractRejectsStaleExtractionAfterClear -count=1 -v ./...` | 1 | SQLite old-generation post-clear `AddMemory` returned nil; PostgreSQL skipped as `blocked-env`. |
| `tools/trpc-native-p1/identityprobe` | `GOWORK=off go test ./... -count=1` | 0 | local key/cursor encoding contract passed. |
| same | `GOWORK=off go test -race ./... -count=20` | 0 | no race reported. |
| same | `GOWORK=off go vet ./...` | 0 | no findings. |
| repository root | `GOWORK=off go test ./internal/application/service -run TestExecuteDurableRunPersistsBudgetExhaustionForNotification -count=1 -v` | 1 | Only `run_started` and `run_failed` persisted; required `budget_exhausted` event was absent. |
| repository root | `GOWORK=off go test ./internal/agent/recoverytest -run 'TestCrashMatrixPostgreSQL|TestTwoWorkerContentionPostgreSQL|TestCrashAfterToolResult' -count=1 -v` | 0 | all three required cases skipped: `TRPC_RECOVERY_PG_DSN` and `TRPC_RECOVERY_GRAPH_PROVIDER` are unset; classify `blocked-env`, never pass. |

The Session forced RED uses a response event with exactly the same
`event.Event.ID`, restoring its timestamp and changing only assistant content.
The Memory forced RED clears the subject scope and then commits stale extraction
content from the prior generation.  Those are the required negative cases,
not hypothetical source-only risks.

### Probe RED assertion boundary

The forced probe REDs establish only that the **raw** backend returned no error
for a changed stable Event.ID or an old-generation post-clear write.  They do
not assert a controlled-facade typed `conflict`/CAS error, do not reopen and
compare the durable changed payload, and do not reopen and prove an empty
Memory scope after the stale write.  They are therefore rejection evidence for
the raw candidates, never GREEN evidence for the selected facade.  P1.3 owns
the Session RED/GREEN cases for typed conflict plus reopened payload/receipt;
P1.4 owns the Memory RED/GREEN cases for generation/tombstone CAS plus reopened
empty-set/retained-metadata assertions.  Probe files are not product tests and
are not changed by this decision.

## Frozen transaction, CommitIntent, barrier, and reconcile contract

`interfaces.md` already defines the required P1 types without a backend-specific
difference, so it is deliberately unchanged by P1.0.  P1.1 must materialize
that exact contract rather than revise it per backend:

1. A business-database transaction validates current scope, grant, fence and
   plan; it records admission/attempt/approval before dispatch.  Failure here
   means zero external calls.
2. After a confirmed result, one transaction records the immutable result,
   `CommitIntent`, checkpoint/pending writes, Session appends, business-event
   intents and usage intents.  The checkpoint is non-runnable until the
   barrier succeeds.
3. The controlled Session facade atomically deduplicates on
   `(AppName, UserID, SessionID, StableEventID)` with the frozen `PayloadHash`:
   same ID/hash is idempotent; same ID/different hash is `conflict`.  It records
   each durable apply before `CommitIntent` becomes Applied.
4. `Barrier` verifies Applied intent, current fence, every result reference and
   every Session receipt before advancing a checkpoint, allocating event
   sequence, emitting durable business events, or finalizing a run.
5. `Reconcile` retries unfinished local intents idempotently after a crash.  It
   never redispatches a confirmed external call.  Unknown non-idempotent effect
   becomes `waiting_user`, not automatic retry.
6. Memory enqueue, transcript read, extraction and final write each recheck
   authorization/settings and compare current generation/tombstone in the same
   write transaction; an old job must fail after clear/delete/disable.

The current SDK Runner logs a Session append failure and may still notify
completion.  Its completion event is therefore never a barrier.  An append or
checkpoint failure must produce `durable_store_unavailable`, latch execution,
and prevent later model/tool/graph dispatch until reconciliation succeeds.

## Six recovery gaps mapped to ownership

This table expands the six rows in `recovery-gaps.md`; it is not a replacement
for the transaction protocol above.  “Unknown” always means no automatic
redispatch of a non-idempotent external effect.

| Recovery gap | Current gap | Target CommitIntent / barrier / reconcile behavior | Unknown side-effect strategy | Responsible stages | Required concrete evidence |
| --- | --- | --- | --- | --- | --- |
| Model result → tool-plan commit | Runner callbacks do not atomically persist provider tool-call ID, args hash, model attempt and fence before dispatch. | A pre-dispatch business transaction records one validated plan journal; the dispatch guard verifies its durable record and current fence before the first call. CommitIntent/Barrier are not created for this step. | No external call exists yet: persistence failure means zero dispatch. | P1 contract, P2 plan/journal, P3 runner integration, P7 acceptance. | SIGKILL after plan-journal commit; SQLite and PostgreSQL reopen show one unchanged plan/args hash; real-provider matrix separately proves provider IDs. |
| Approval → external call | `graph.Interrupt` exists but raw callbacks do not make the human decision durable or recheck it at dispatch. | A pre-dispatch business transaction binds plan, approver, scope/revision and expiry; the dispatch guard rechecks that durable approval and fence immediately before the call. CommitIntent/Barrier apply only after a confirmed external result. | Invalid, revoked or expired approval gives `waiting_user` with zero dispatch. | P1 decision contract, P2 approval gate, P3 dispatch integration, P7 acceptance. | SIGKILL after approval-journal commit before call; test wrong subject, expiry and revocation with exactly one compliant dispatch. |
| Call success → result persistence | External success can precede local result/receipt storage. | CommitIntent stores immutable result, provider receipt/query anchor and effect state; Barrier requires the result receipt; Reconcile reuses a confirmed result. | Query if supported; idempotent retry only with proof; otherwise `unknown_effect` and `waiting_user`. | P1 result model, P2 query/idempotency policy, P3 transition, P7 real-provider/connector acceptance. | SIGKILL after success before result write; demonstrate query/reuse for queryable calls and no redispatch for unqueryable non-idempotent calls. |
| Result persistence → checkpoint | Existing SQLite probe proves a pending write, not result-reference/checkpoint atomicity. | CommitIntent carries result references and non-runnable checkpoint pending writes; Barrier makes checkpoint runnable only after references verify; Reconcile completes the same intent. | A confirmed result is never called again; missing/invalid reference blocks recovery. | P1 invariant, P2 recovery validation, P3 saver, P7 crash matrix. | Kill once after result commit and once after checkpoint commit; SQLite/PostgreSQL reopen prove no repeat call and matching references/pending order. |
| Checkpoint → Session | Current checkpoint evidence does not coordinate new Session history authority. | CommitIntent contains a Session append intent; the controlled facade separately records the durable `(AppName, UserID, SessionID, StableEventID, PayloadHash)` apply receipt. Barrier requires that receipt before advancing; Reconcile applies the same intent idempotently. | No external call is inferred from Session state; conflict or append failure latches execution. | P1 authority contract, P1.3 Session facade, P3 recovery integration, P7 race/process acceptance. | Cross-process and two-tenant reopen on SQLite/PostgreSQL; same ID/same hash succeeds, changed hash returns typed conflict, append failure causes zero later dispatch. |
| Event persistence → client send | Current `emit` can warn and continue; a sent client message is not durable state. | CommitIntent records business event/outbox; Barrier allocates sequence only after durable append; Reconcile replays the outbox, and client sends read committed sequence. | Client disconnect has no external-effect implication; reconnect replays from `Last-Event-ID`; append failure remains visible/recoverable. | P1 event contract, P1.5 event implementation, P3 projection, P5 protocol, P7 client acceptance. | Kill/interrupt around append, projection and send; reconnect reads ordered no-loss/no-duplicate events; append failure stops dispatch and exposes recovery. |

## Unlock conditions and executable evidence tasks

P1.0 now unlocks P1.1 and, after P1.1 is integrated, P1.2.  P1.2 implements
the selected adapter/migrations for SQLite and PostgreSQL, pins its exact
resolved graph, and runs its scoped RED/GREEN/review sequence.  Its completion
does not pre-approve P1.3/P1.4/P1.5/P2: each proceeds only after its explicit
DAG predecessor and task-level evidence.  The required later evidence is
assigned rather than made a self-blocking P1.2 precondition:

1. P1.3 proves Session scope/reopen/list/delete, receipt uniqueness,
   same-ID/same-hash idempotency, typed same-ID/different-hash conflict and
   append-failure zero-dispatch on both active dialects.
2. P1.4 proves clear/delete/disable/re-enable against delayed extraction,
   authorization recheck and generation/tombstone CAS, including reopened
   empty-memory and retained business-metadata assertions.
3. P1.5 proves CommitIntent, barrier, reconcile, outbox/event ordering and
   the corresponding persistence failures.  P2 adds approval, budget and
   side-effect policy; the currently failing `budget_exhausted` notification
   test is a separate repair and regression obligation.
4. P7 runs the six-gap process-kill/two-worker acceptance matrix on SQLite and
   PostgreSQL, plus real-provider/connector and client replay evidence.  It
   requires `TRPC_RECOVERY_PG_DSN` and an authorized
   `TRPC_RECOVERY_GRAPH_PROVIDER`; until supplied those cases are `blocked-env`.

P3 native Runner wiring, P4/P5 clients, and release remain controlled by the
original P0 conditions plus their own acceptance.  Nothing here changes P0
from `NO-GO` to `GO`.

## P0 ruling retained

P0's `NO-GO` is unchanged because its required persistence composition,
append-failure barrier, budget-notification consumer, PostgreSQL recovery and
real-provider evidence are not complete.  The P0 decision document and
`interfaces.md` require no edit: this document records the exact new P1.0
evidence without falsely elevating it to product or release acceptance.
