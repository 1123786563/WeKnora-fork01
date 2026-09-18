# W23 implementation report

## Scope

Implemented personal-node outbound enrollment and revocation on base `05f849e8` in worktree `.sdd-worktrees/w23`.

The plan names `000129`/`000049` for this task, but those migration numbers already belong to `craft_sessions` in the current repository. To preserve migration ordering and avoid overwriting an accepted migration, this implementation uses the next free numbers `000136` (PostgreSQL) and `000058` (SQLite). This ruling must be recorded by the coordinator before integration.

## Changes

- `services/paseo-adapter/src/node-connector.ts`: grant validation, challenge/registration client contracts, owner-scoped connector, revoked-credential fencing, close-on-revoke, and bounded reconnect backoff.
- `internal/execution/registration.go`: Ed25519 proof-of-possession, one-time challenge consumption, tenant/owner/idempotency scoping, credential version bump on revoke, and `NodeGrant` validation.
- `internal/handler/execution_registration.go`: authenticated challenge/complete/revoke HTTP handlers; no bearer, private key, root path, or address accepted.
- `internal/router/{router.go,routes_workbench.go}` and `internal/container/container.go`: real DI and authenticated route wiring.
- PostgreSQL/SQLite migrations for challenge and registration state. Only public-key fingerprints and short-lived challenge material are persisted; long-lived private credentials are never stored.
- Focused Go/TypeScript tests for old epoch, expiry, proof-of-possession, one-time challenge, idempotent replay/conflict, tenant/owner isolation, revocation version bump, connector fencing, and reconnect backoff.

## Evidence

| Layer | Command | Result |
| --- | --- | --- |
| Go behavior | `go test ./internal/execution -run 'TestValidateNodeGrant\|TestRegistration' -count=1` | PASS |
| Paseo adapter behavior | `pnpm exec tsx --test services/paseo-adapter/src/node-connector.test.ts` | PASS, 3/3 |
| TypeScript | `pnpm --filter @weknora/paseo-adapter typecheck` | PASS |
| Static diff | `git diff --check` | PASS |
| Database migration suite | `go test ./internal/database -run 'TestSQLite.*Migration\|TestMigration' -count=1` | BLOCKED by existing SQLite migration 000055 nested-transaction failure |
| HTTP/package compile | `go test ./internal/handler -run 'TestExecutionRegistration' -count=1` | BLOCKED by unrelated W20 `remote_dispatch.go` field/method compile conflict |
| Live node/provider | personal Paseo node | NOT RUN; no authorized live node/provider environment |

## Known limits

- The registration service is wired to the application route, but full HTTP/package verification waits for the existing W20 compile failure to be repaired.
- PostgreSQL migration/runtime evidence and live personal-node/Paseo evidence remain unavailable in this environment and must be recorded as `blocked-env` rather than inferred from unit tests.
- Independent reviewer must verify migration-number ruling, cross-tenant owner predicates, replay conflict behavior, and the absence of secret persistence before integration.
