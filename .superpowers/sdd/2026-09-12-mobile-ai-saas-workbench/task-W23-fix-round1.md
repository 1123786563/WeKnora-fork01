# W23 targeted fix round 1 implementation report

- **Base:** `59b3c7f1`
- **Head:** `d7657d26`
- **Scope:** contract revoke fencing, Paseo composition, admission coverage, SQLite/Gin lifecycle evidence

## Changes

- `ExecutionTargetStore.RevokeTarget` now runs a transaction that locks the owned active target, increments its credential version, and for `personal_node` targets revokes the registration and identity projections with exact row-count checks. Missing projections roll the transaction back; managed targets retain target-only semantics.
- Added repository tests for personal projection revocation, credential increments, idempotent second revoke, and rollback when a projection is missing.
- Expanded W20 admission tests for tenant, owner, workspace-target, zero credential, and revoke-between-initial/final target resolution.
- Added a SQLite-backed Gin registration → complete → contract revoke integration test, asserting registration/identity state and version plus second-revoke error mapping.
- Added `createPersonalNodeConnector` production composition seam. It injects the configured/allowlisted Paseo transport, reads the bearer from an OS-backed credential store, clears it after revoke, and invokes lifecycle cleanup.

## Verification

- `pnpm exec tsx --test services/paseo-adapter/src/node-connector.test.ts` — PASS, 5/5.
- `pnpm --filter @weknora/paseo-adapter typecheck` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools GOWORK=off go test -race ./internal/execution -run 'Test(Registration|ValidateNodeGrant)' -count=1` — PASS.
- `git diff --check` — PASS.
- `go test ./internal/application/repository -run 'TestExecutionTargetStore' -count=1 -v` — BLOCKED by pre-existing W20 `internal/application/service/workbench/remote_dispatch.go` field/method `dispatch` conflict.
- `go test ./internal/handler -run 'TestExecutionTargetRegistrationGinSQLiteLifecycle|TestExecutionRegistration' -count=1 -v` — BLOCKED by the same W20 compile failure before tests execute.
- PostgreSQL runtime and live Paseo node evidence — blocked-env; no authorized environment available.

## Review request

Independent reviewer must verify the transaction boundary, managed-target compatibility, route behavior, SQL fixture fidelity, and whether the TS factory is sufficient production composition rather than test-only construction.
