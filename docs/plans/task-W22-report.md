# W22 implementation report

## Scope

W22 adds the control boundary for Paseo-backed runs: durable product cancellation remains separate from provider process termination; stop confirmation requires a fresh `exited` or `destroyed` observation; approval forwarding carries the external pending id, argument digest, credential version, and optimistic revision; and workspace leases are tenant-scoped and owner/epoch fenced.

## Changes

- `internal/execution/control.go`: `ExecutionObservation`, `MayReleaseWorkspace`, `StopRemoteExecution`, and owner/epoch-fenced process-local lease primitive.
- `services/paseo-adapter/src/control.ts`: strict cancel/steer/submitInteraction command envelope and cancel/observe reconciliation.
- `internal/application/service/workbench/interaction.go`: durable GORM workspace lease store, remote interaction seam after the W05 CAS, and persisted approval metadata projection.
- `internal/application/service/agent_run_lifecycle.go`: remote cleanup uses a bounded context detached from a disconnected HTTP request and reports `remote_stop_unconfirmed` separately from product cancellation.
- migrations `000136` / SQLite `000058`: interaction approval metadata and durable workspace lease table.

## Evidence

- `pnpm --filter @weknora/paseo-adapter test`: 26 passed.
- `pnpm --filter @weknora/paseo-adapter typecheck`: passed.
- `go test -race ./internal/execution -run 'Test(CanceledRun|StopRemote|WorkspaceLease)' -count=1`: passed.
- `go vet ./internal/execution`: passed.
- `go test ./internal/application/service/workbench -run 'Test(GormWorkspaceLease|InteractionService|GormInteractionStore)' -count=1`: passed after the pre-existing W20 `RemoteDispatcher` field/method collision was temporarily corrected in the test checkout.
- `go test ./internal/application/service -run 'TestAgentRunLifecycle(RemoteCancel|CancelIsDurable)' -count=1`: passed under the same temporary W20 compile correction.
- `git diff --check`: passed.

## Environment blockers

The current W20 merge base contains a compile error in `internal/application/service/workbench/remote_dispatch.go`: `RemoteDispatcher` declares a `dispatch` field and a `dispatch` method. It is outside W22 scope and was restored unchanged after focused verification. Full packages depending on that file therefore require the W20 repair first. Full migration-suite execution also remains blocked at the existing SQLite 000055 transaction failure before W22 migration 000058 is reached.
