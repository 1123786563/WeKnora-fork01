# Final Fix Report — Ticket #58

Branch: `marketplace/t28-tenant-catalog-release`

## Fix wave

- Bound `repository.AgentMarketplaceRepository` to the exact
  `interfaces.AgentMarketplaceRepository` required by the marketplace service.
- Added a tenant/submission unique review constraint in both SQLite and
  PostgreSQL migrations. Same decision retries still return the durable
  result; a competing review returns a conflict.
- Enforced SemVer and required, bounded display names while building the
  immutable bundle.
- Mapped validation, missing named immutable dependencies, not-found,
  stale-digest, pointer, and review conflicts to structured HTTP errors.
- First successful approval now initializes Listing display metadata from its
  approved Manifest. Rejected submissions do not seed the catalog.
- Review queue responses parse the persisted canonical bundle and expose its
  fixed payload. Contracts parse it and the review UI renders it.

## Evidence

| Command | Result |
| --- | --- |
| `go test ./internal/application/repository ./internal/application/service ./internal/agent/experts ./internal/router ./internal/container ./internal/database -run 'AgentMarketplace|AgentRelease|TenantRelease|SQLiteMigrations|BuildContainer' -count=1` | exit 0; container linker emitted existing `-lc++` duplicate-library warning |
| `pnpm --filter @weknora/contracts exec tsx --test src/marketplace/tenant-releases.test.ts` | 3 passed |
| `pnpm --filter @weknora/api-client exec tsx --test src/marketplace/tenant-releases.test.ts` | 3 passed |
| `pnpm --filter @weknora/web exec tsx --test src/agent-marketplace/TenantReleaseReview.test.tsx` | 3 passed |
| `git diff --check` | exit 0 |

## Residuals

- PostgreSQL migrations were reviewed for parity but no PostgreSQL DSN is
  available in this environment.
- The existing linker warning is unrelated to this wave.
