# A03 Task 2 report — capability and invocation ledger

## Commits

- `9a36fc5e feat(semantic): add model capability invocation ledger`
- `93767c43 fix(semantic): serialize invocation task quota claims`

## Delivered files

- Go-only capability and tightly bounded wire types: `internal/types/semantic_model.go` and `internal/types/interfaces/semantic_model.go`.
- HS256 A03 issuer/verifier using a distinct `SEMANTIC_MODEL_SIGNING_KEY`: `internal/application/service/semantic_model_capability.go`.
- Owner-run invocation ledger with claim, completed replay, known pre-dispatch release, and unknown retention: `internal/application/repository/semantic_model_invocation.go`.
- PostgreSQL `000180` and SQLite `000101` formal migrations, plus updated migration-cycle coverage.
- Sensitive configuration/startup-banner registration and container repository construction.

## Evidence

| Command | Result |
| --- | --- |
| `go test ./internal/application/repository ./internal/application/service ./internal/config -run '^TestSemanticModel(Capability|Invocation)' -count=1` | PASS |
| `go test ./internal/application/repository -run '^TestSemanticModelInvocation' -count=1` | PASS |
| `go test ./internal/config -count=1` | PASS |
| `go test ./internal/container -run '^$' -count=1` | PASS |
| `go test ./internal/database -run '^TestSemanticMigrationSQLiteUpDownUp$' -count=1` | PASS (SQLite up/down/up) |
| `git diff --check` | PASS before commit |

The PostgreSQL migration test requires `TRPC_TEST_POSTGRES_DSN`; this worktree did not have it configured. It was not executed, and no DSN value was read or logged.

## Rulings and self-review

- Invocation identity is server-derived: `semantic-run:` is SHA-256 of the exact A01 scope ref; `semantic-call:` hashes the scope ref plus opaque invocation key. The key is not an authority input.
- Ledger claims lock the owner run row on PostgreSQL before aggregate quota validation; SQLite lock/duplicate races retry and converge on the same call row. Completed rows replace the reserved envelope with observed usage; unknown rows retain their maximum envelope; known pre-dispatch failure is excluded from quota.
- Capability issuance/verification re-resolves the live A01 scope and reloads enabled owner policy plus immutable pricing before any store/budget mutation. Platform task budget registration is owner-tenant only; BYOK does not invoke the commercial budget store.
- Task 2 intentionally stops before provider dispatch and before HTTP routes. The production policy/pricing resolver remains fail-closed, so this capability issuer is not yet assembled into a runnable gateway; Task 3/4 own gateway assembly and route exposure.
- The implementation stores no token or provider credential. A03 model signing key is hidden from YAML/JSON and is logged by startup only as sensitive presence/length metadata.

## Review remediation — round 1

Addressed the four Task 2 review findings:

- `SemanticModelInvocationStore.Claim` now returns an explicit `SemanticModelInvocationClaim` disposition. Only `claimed_new` authorizes provider I/O; `in_flight` and `unknown` are non-dispatch outcomes, while `completed_replay` carries the persisted result.
- Concurrent `EnsureRun` uses `INSERT ... ON CONFLICT DO NOTHING`, then verifies every persisted binding field. Identical concurrent requests converge; a differing binding remains a conflict.
- Capability verification passes `jwt.WithExpirationRequired()` and validates the issued-at claim when present. Tests cover expired, missing-expiry, and wrong-audience tokens without a ledger or budget dependency.
- Capability issue rejects an input-token envelope above `math.MaxInt64/4` before deriving `max_input_bytes`; the exact boundary is accepted and produces the checked positive bound.

Covering tests: `internal/application/repository/semantic_model_invocation_test.go` and `internal/application/service/semantic_model_capability_test.go`.

| Command | Observed result |
| --- | --- |
| `go test ./internal/application/repository ./internal/application/service ./internal/config -run '^TestSemanticModel(Capability|Invocation)' -count=1` | PASS — repository, service, and config packages all exit 0 |

The controller independently verified isolated PostgreSQL migration up/down/up with `go test -tags semantic_integration ./internal/database -run '^TestSemanticPostgresMigrationUpDownUp$' -count=1` (PASS); its DSN remains private. SQLite migration cycling was retained from the original Task 2 evidence.
