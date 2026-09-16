# W20 implementation report

## Scope

W20 adds a durable provider-dispatch intent/receipt boundary. The provider is
never called before a `claimed` intent is committed, an uncertain response is
recorded as `unknown`, and only an observed external id can move a record to
`reconciled`/`completed`. Claims, receipts, and reconciliation recheck the
tenant, run, and current run epoch inside one transaction.

The task branch was created from `b80882db` (W19). The W04 accepted commit
`091f1cc0` was not an ancestor of that branch, so its four accepted test files
were temporarily cherry-picked as the inherited W04 baseline (`685ee773`).
They are outside the W20 implementation diff and must not be counted as W20
scope when the coordinator creates the review package.

## Files

- `internal/application/repository/execution_dispatch.go`
- `internal/application/repository/execution_dispatch_test.go`
- `internal/application/service/workbench/remote_dispatch.go`
- `services/paseo-adapter/src/command-log.ts`
- `services/paseo-adapter/src/command-log.test.ts`
- `migrations/versioned/000098_execution_dispatch.{up,down}.sql`
- `migrations/sqlite/000020_execution_dispatch.{up,down}.sql`

The plan's original migration names `000127`/`000047` were renumbered to the
actual current heads reserved by W05: PostgreSQL `000098` after `000097`, and
SQLite `000020` after `000019`.

## Verification

- `pnpm exec tsx --test services/paseo-adapter/src/command-log.test.ts` — PASS, 5 tests.
- `pnpm --dir services/paseo-adapter typecheck` — PASS.
- `go test ./internal/application/repository -run 'TestExecutionDispatch' -count=1` — PASS, 3 tests.
- SQLite migration up/down and repository fixture migration — PASS as exercised by the repository test helper.
- `go test ./internal/execution` — PASS.

The package-level `go test ./internal/application/service/workbench` remains
blocked by the inherited W04 branch state: `repository.WorkbenchRequest*` is
not present in the W19 base. This is an inherited dependency gap, not a W20
compile failure in `remote_dispatch.go`; it must be re-run after integrating
the complete W04/W05 chain. A live Paseo node/credential was not available,
so real-provider exactly-once and process-count evidence is `blocked-env`.

## Review fix round 1

- Expired claims are atomically converted to `unknown`; unknown claims cannot
  be started again. Only `RecoverUnknown` after an explicit `not_started`
  observation can issue a new lease.
- `ClaimDispatchWithPayloadHash` binds an immutable payload hash to the command
  id and rejects a different hash.
- `DispatchRecord` carries the worker/lease token. `SaveReceipt` requires the
  current worker, epoch, and unexpired lease, so an old fence cannot commit a
  receipt.
- `RemoteDispatcher` now requires a concrete `RemoteProvider`; nil providers
  fail closed and no callback can fake a successful start.
- Added 20-way same-command contention, hash conflict, cross-tenant,
  lease-expiry/unknown recovery, stale-fence, and SQLite head assertions.

Fix-round verification:

- `go test ./internal/application/repository -run '^TestExecutionDispatch' -count=1` — PASS.
- `go test -race ./internal/application/repository -run '^TestExecutionDispatch' -count=1` — PASS.
- `go test internal/application/service/workbench/remote_dispatch.go` — PASS (file-level compile; the full inherited package remains blocked as described above).
- Paseo command-log tests and adapter typecheck — PASS.
- The full repository package suite still has an unrelated pre-existing SQLite
  credential-rotation lock failure in `TestCreateTargetIfTrustedConcurrentCredentialRotation`; it is recorded as a broader-suite environment/concurrency failure and is not counted as W20 targeted evidence.

## Review fix round 2

- `ReconcileUnknown` now requires the current dispatch worker, matching run
  epoch, and an unexpired dispatch lease. An old claimant cannot overwrite a
  replacement claim after `RecoverUnknown`; an inconclusive observation keeps
  the current lease for the same worker's lookup, while an identified receipt
  closes it.
- `AgentRunWorker` now has a driver-aware scan/claim path and an explicit
  `NewAgentRunWorkerWithRemoteDispatch` entrypoint. `container.NewAgentRuntimeWithRemoteProvider`
  assembles the real W20 dispatcher/provider boundary; the provider is invoked
  only after the durable claim and receipt is persisted before post-start
  execution. The worker integration fixture verifies the Paseo driver path and
  persisted `completed` receipt (the fixture dispatcher is local because this
  branch lacks the complete W04 admission repository chain).
- SQLite acceptance fixtures now treat W04=017, W18=018, W05=019, and W20=020
  as the authoritative chain. Down/up and later-migration transaction tests
  pass against the full chain.

Fix-round-2 verification:

- `go test ./internal/application/repository -run '^TestExecutionDispatch' -count=1` — PASS.
- `go test -race ./internal/application/repository -run '^TestExecutionDispatch' -count=1` — PASS.
- `go test -race ./internal/application/service -run 'TestAgentRunWorker(RemoteDispatchUsesDurableIntent|TickSkipsPaseo)' -count=1` — PASS.
- `go test ./internal/database -count=1` — PASS.
- Paseo command-log tests — PASS, 5 tests; adapter typecheck — PASS; `git diff --check` — PASS.
- Full `internal/application/service/workbench` package remains `blocked-env` on
  this isolated W19 base because `repository.WorkbenchRequestRepository` and
  its constructor/model are not present; PostgreSQL DSN and live Paseo
  credentials remain unavailable and are not claimed as runtime acceptance.

## Review fix round 3

- `BuildContainer` now registers one `ExecutionDispatchStore` beside the
  durable run repositories and resolves `AgentRuntime` through
  `NewAgentRuntimeWithRemoteProvider`, so the production construction path
  cannot silently select the legacy worker.
- This isolated W19 base has no Go Paseo provider implementation or provider
  factory to inject. The constructor passes a typed nil provider for
  unconfigured deployments and fails closed when durable recovery is enabled;
  it never reports a provider dispatch as successful. The concrete Paseo host
  must supply the provider at this integration seam before enabling recovery.
- Verification is limited by the inherited branch gaps documented above:
  `go test ./internal/container` cannot compile because
  `repository.WorkbenchRequestRepository` is absent, while recovery config
  fields arrive only with the later config-chain commits. `git diff --check`
  passes for this scoped fix.

## Review fix round 4

- Added `RemoteStartRequest` and the optional `RemoteCommandProvider` port.
  Dispatch now forwards the worker fence epoch and immutable payload hash;
  concrete providers receive fenced command data instead of relying on a
  global prompt or hard-coded epoch.
- The Paseo HTTP adapter implements `StartCommand`, validates command fields,
  and sends the versioned bridge envelope with epoch, attempt, prompt, target,
  workspace, provider, and payload hash. The bridge remains responsible for
  admission/HMAC verification; bearer transport alone is not authorization.
- Targeted Go execution was attempted, but the local cgo toolchain is blocked
  by the host Xcode license gate. This is recorded as `blocked-env`; no live
  Paseo acceptance is claimed.
