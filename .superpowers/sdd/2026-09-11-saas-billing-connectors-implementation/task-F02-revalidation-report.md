# F02 revalidation implementation report

## Scope

This revalidation closes the missing PostgreSQL concurrency-test evidence for
the existing F02 tenant-to-Customer mapping. It adds only the test-tagged
`internal/application/repository/commercial/account_pg_test.go`; production
code and F03-or-later scope are unchanged.

The source `task-2-brief.md` was empty in this worktree. The task contract
used here is the assigned F02 revalidation requirement: exercise real
PostgreSQL with `AccountStore` and the production account migration, and prove
that exactly one bind succeeds for each conflicting pair.

## Test-first evidence

No valid behavior RED was captured for this revalidation. The production
`AccountStore` and its SQLite coverage existed before this evidence-only test
addition. The earlier deliberate absence of `testAccountPGStore` produced a
compile failure, but that only demonstrated a missing test fixture, not a
failing product behavior; it must not be represented as an expected RED.

The test-side fixture then added an isolated PostgreSQL schema, applied
`migrations/versioned/000110_commercial_accounts.up.sql`, and created the real
`AccountStore`. The two tests cover concurrent binds for one Tenant with two
Customers and two Tenants with one Customer. Each asserts exactly one success
and requires every loser to return `ErrAccountMappingConflict`.

## GREEN and verification

| Command | Actual result |
| --- | --- |
| `go test ./internal/commercial ./internal/application/repository/commercial -run 'Test(BillingAccess|Account)' -count=1` | PASS: both packages returned `ok`. |
| `go test -tags commercial_integration -v ./internal/application/repository/commercial -run '^TestAccountPG' -count=1` | `TestAccountPGConcurrentSameTenantDifferentCustomers` and `TestAccountPGConcurrentDifferentTenantsSameCustomer` each SKIP with `blocked-env: SAAS_TEST_PG_DSN is required`. This is not a PostgreSQL runtime pass. |
| `go vet ./internal/commercial ./internal/application/repository/commercial` | PASS (no output, exit 0). |
| `go build ./internal/commercial ./internal/application/repository/commercial` | PASS (no output, exit 0). |
| `gofmt -l internal/application/repository/commercial/account_pg_test.go` | PASS (empty output). |
| `git diff --check` | PASS (empty output). |

## Commit and review state

- Implementation commit: `af8be5e15c7739b14bbc841562fc9bf7fe02940e` (`test(commercial): add PostgreSQL account uniqueness evidence`).
- Independent review: pending.

## Environment limitation

`SAAS_TEST_PG_DSN` was absent. The integration fixture is intentionally
test-tagged and calls `t.Skip("blocked-env: SAAS_TEST_PG_DSN is required")`
when absent, so no PostgreSQL connection, migration, or concurrent database
execution was claimed. Supply an isolated PostgreSQL DSN and rerun the tagged
command above for runtime evidence.

## Review fix

Following independent review, fixture cleanup is centralized and checks every
resource-release error with `t.Errorf`: the isolated schema pool closes before
`DROP SCHEMA ... CASCADE`, the schema drop is checked, and the PostgreSQL admin
pool close is checked. Cleanup is registered before schema creation, so the
same checked cleanup applies when isolated-connection initialization fails
after schema creation. This preserves cleanup safety without changing
production behavior.
