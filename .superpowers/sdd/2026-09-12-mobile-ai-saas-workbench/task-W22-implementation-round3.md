# W22 round-3 implementation report — `261a4143`

## Scope

Implemented the round-2 review fixes for remote lifecycle fencing, provider observations, approval command idempotency, worker cancellation, and the Go/TypeScript control response contract.

## Changes

- Added workspace lease lifecycle integration to the remote worker: admission lease handles are renewed with the worker heartbeat and released only after the provider confirms a fenced remote exit. Cancellation snapshots active state under the worker mutex.
- Added durable dispatch receipt lookup by tenant/run. Cancellation after worker restart uses the persisted external receipt and the same epoch to stop and observe the provider.
- Resolved the target's current positive `credential_version` through the owned target store before execution and carried the trusted snapshot through the worker context into tool approval metadata.
- Added provider epoch to the bridge observation response and reject missing/mismatched epochs. The TypeScript control contract now requires an epoch for every observation and no longer accepts an absent epoch.
- Made approval control command IDs deterministic from the durable decision ID and removed tenant/owner fields from the fixed TypeScript `submitInteraction` payload. The service passes the durable decision ID through the remote interaction port.
- Generated a canonical non-empty SHA-256 payload hash in the assembled remote runtime command builder.
- Added a Go HTTP fixture covering the fixed control accepted response and provider epoch observation.

## Validation

PASS:

- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test -race ./internal/execution -count=1`
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test -race ./internal/container -run TestPaseoRemoteProvider -count=1`
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go vet ./internal/execution ./internal/application/service/workbench ./internal/application/service ./internal/container`
- `pnpm --filter @weknora/paseo-adapter typecheck`
- `pnpm --filter @weknora/paseo-adapter test` — 26/26
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test ./internal/execution -run 'TestBridge(ControlAndObservationEpochFixture|FixedProtocol)' -count=1`
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools git diff --check`

BLOCKED-BASELINE:

- Worker integration tests that initialize the complete SQLite schema remain blocked by the pre-existing migration 000055 nested-transaction failure, which leaves the test database dirty at version 55 before the W22 migration. This is recorded as an environment/baseline blocker and is not counted as W22 pass evidence.
- No live Paseo daemon/provider credentials were available; live create/observe/cancel/approval confirmation remains unproven.
