# P1.0 storage-decision — tRPC Native Agent

Decision date: 2026-09-19.  Base examined: `f5caa23eef892b3b33d8e1b62398152868fd95a5`.
This is an evidence and contract decision.  It neither enables the native
Runner nor authorizes a production migration, new wiring, or a change to the
P0 product-execution ruling.

## Decision

**P1 storage composition is `blocked-design`; native Runner product execution
remains P0 `NO-GO`.**  No raw tRPC Session or Memory SQL backend is selected
for product use.  The only selected design boundary is a WeKnora-controlled
Session/Memory facade backed by the same business database used for the run,
commit-intent, business-event, and governance records.  It may call an SDK
service behind that facade only after the P1.2 implementation proves every
contract below for each active dialect.  This is a boundary selection, not a
physical backend approval and not permission to implement P1.2.

| Candidate | Decision | Evidence and consequence |
| --- | --- | --- |
| Root `trpc-agent-go v1.11.0` | retained SDK API candidate; **not** an approved product composition | The independent probes compile against it and their default suites pass, but the root only has a bounded repeated Runner race green.  Direct-consumer budget notification remains failing and the storage contract is not met. |
| `session/sqlite v1.11.0` | rejected as a raw product backend | Replaying a stable `event.Event.ID` with changed payload returned nil in the forced contract test.  Duplicate stable IDs are stored; an append can mutate the caller object before persistence fails. |
| `session/postgres v1.11.0` | rejected as a raw product backend | Historical isolated-DSN characterization has the same stable-ID conflict failure and default schema initialization fails its unique-index check.  This run had no DSN, so PostgreSQL re-verification is `blocked-env`. |
| `memory/sqlite v1.11.0` | rejected as a raw product backend | A clear followed by old-generation extraction output returns nil in the forced contract test; no generation/tombstone/CAS input exists on raw `AddMemory`. |
| `memory/postgres v1.11.0` | rejected as a raw product backend | Historical isolated-DSN characterization has the same stale-write behavior.  This run had no DSN, so PostgreSQL re-verification is `blocked-env`. |
| WeKnora controlled Session/Memory facade and repository | selected target boundary, **not yet implemented** | It is the minimum boundary that can enforce authoritative scope, receipt/hash uniqueness, generation/tombstone CAS, business metadata, transaction coordination, and authorization recheck without using the SDK as an authorization service. |

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
Memory probe's explicitly resolved `v1.11.0`.  That inconsistency is itself a
reason no combined raw SDK SQL composition is approved.  P1.2 must pin and
verify its complete resolved graph instead of borrowing either probe's graph.

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

## Unlock conditions and executable evidence tasks

P1.1 may freeze the already-defined `nativecontract` types only after this
document is reviewed and integrated.  P1.2 and every downstream implementation
remain `blocked-design` until all of the following have evidence on its exact
commit and resolved module graph:

1. Implement the selected controlled facade and its SQL migrations for both
   PostgreSQL and SQLite.  Test scope isolation, reopen/list/delete, receipt
   uniqueness, same-ID/same-hash idempotency and same-ID/different-hash
   conflict.  Inject an append failure and prove zero later dispatch.
2. Supply disposable PostgreSQL DSNs and run the Session and Memory probe
   characterization plus forced contract commands with `P1_*_PG_DSN` and
   `P1_REQUIRE_CONTRACT=1`; no skipped PostgreSQL case may be reported as pass.
   Resolve the Session PostgreSQL initializer/index defect or avoid it through
   the reviewed facade migration, then prove the chosen path.
3. Test Memory clear/delete/disable/re-enable against delayed extraction writes
   on both dialects; prove old generation and revoked subject writes fail and
   all retained business metadata survives.
4. Add six-gap crash/reconcile tests around intent creation, result persistence,
   Session apply, barrier, event projection and finalization.  Include two
   workers, stale fence, and cross-process recovery.  A database transaction
   does not claim external exactly-once behavior.
5. Repair and regression-test
   `TestExecuteDurableRunPersistsBudgetExhaustionForNotification`: persist the
   durable `budget_exhausted` event before notification projection, then run the
   complete affected service suite.  This is a distinct existing-consumer
   blocker, not a waiver.
6. Provide `TRPC_RECOVERY_PG_DSN` and an authorized
   `TRPC_RECOVERY_GRAPH_PROVIDER`; run PostgreSQL SIGKILL/contention and real
   provider recovery matrices.  Record identity and resources without secrets.

P2 remains blocked by P1's durable store boundary.  P3 native Runner wiring,
P4/P5 clients, and release remain blocked by the original P0 conditions plus
their own acceptance; none of these steps changes P0 from `NO-GO` to `GO`.

## P0 ruling retained

P0's `NO-GO` is unchanged because its required persistence composition,
append-failure barrier, budget-notification consumer, PostgreSQL recovery and
real-provider evidence are not complete.  The P0 decision document and
`interfaces.md` require no edit: this document records the exact new P1.0
evidence without falsely elevating it to product or release acceptance.
