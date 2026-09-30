# Task 3 Report — Privacy-Preserving Aggregated Metrics Projection

## Result

Implemented the fixed Marketplace Metrics projection. The repository counts distinct adopter tenants independently for lifetime introductions, currently active adoptions, all 30-day upgrade proposals, and accepted 30-day upgrades. Local release references are joined to `tenant_introduced_releases` on both `tenant_id` and local `id`. Proposal counts use `created_at` and the half-open UTC interval `[upgradeSince, asOf)`. Adoption and introduction counts are not date-limited. No Evaluation, Task, Run, tool, knowledge-processing, mapping, or diagnostic records are queried.

The service chooses the 30-day UTC window internally and publishes only five explicit bucket/availability fields. Raw aggregate counts remain in the repository package. Error-category availability is fixed to `not_collected` because there is no approved structured marketplace error source in the allowed inputs.

## TDD and verification evidence

- RED: `go test ./internal/application/repository -run '^TestMarketplaceMetricsAggregatesDistinctTenantsAtFixedGrains$' -count=1` initially failed to compile because `NewMarketplaceMetricsRepository` was not yet defined.
- RED: `go test ./internal/application/service -run '^TestMarketplaceMetrics' -count=1` initially failed to compile because the aggregate and service seams were not yet defined.
- Fixture correction: after implementation, the first repository GREEN attempt failed with `NOT NULL constraint failed: tenant_introduced_releases.bundle`; completed both intro fixtures with required columns.
- GREEN: `go test ./internal/application/repository -run '^TestMarketplaceMetrics' -count=5` — PASS.
- GREEN: `go test ./internal/application/service -run '^TestMarketplaceMetrics' -count=5` — PASS.
- `go test ./internal/application/repository ./internal/application/service -count=1` — PASS (`repository` 306.189s, `service` 231.111s).
- `git diff --check` — PASS.
- `go build ./...` — PASS; linker emitted duplicate `-lc++` library warnings for `cmd/desktop` and `cmd/server`.

Tests cover distinct tenant grains and duplicate source rows, 0/4/5/9/10/19/20 bucket thresholds, a 30-day-old active Adoption, interval lower inclusion and upper exclusion, accepted-upgrade counting, UTC normalization, and exact closed JSON output without private marker strings.

## Changed files

- `internal/application/repository/marketplace_metrics.go`
- `internal/application/repository/marketplace_metrics_test.go`
- `internal/application/service/marketplace_metrics.go`
- `internal/application/service/marketplace_metrics_test.go`
- `internal/types/interfaces/marketplace_metrics.go`

## Scope / concerns

No existing interfaces, handlers, containers, migrations, or unrelated source files were modified. The conservative error-category behavior is `not_collected`, as required when no structured approved source exists. No unresolved implementation concern.

## Review Fix R1 — coherent SQL snapshot and coverage fixes

Applied the authorized R1 changes in the three owned files only. The four counts now come from one parameterized `SELECT` with scalar subqueries, so they share one statement snapshot while preserving the tenant/local-id joins and UTC half-open proposal bounds. Boundary proposals now have matching introduction rows. Repository fixtures add a second active Adoption and second accepted Proposal for an existing tenant; distinct tenant counts remain unchanged. The service wire-shape/privacy test now uses SQLite, the real repository, and the real service with introduction/proposal marker values.

### RED evidence

- `go test ./internal/application/repository -run '^TestMarketplaceMetrics' -count=1` before the query refactor — FAIL as intended: statement-count assertion expected 1 statement and observed 4. The boundary rows also demonstrated the need to update the lifetime introduction expected count after adding valid introduction records (observed 22 including those two boundary tenants).

### Exact verification commands and results

- `go test ./internal/application/repository -run 'TestMarketplaceMetrics' -count=5` — PASS: `ok github.com/Tencent/WeKnora/internal/application/repository 0.868s`
- `go test ./internal/application/service -run 'TestMarketplaceMetrics' -count=5` — PASS: `ok github.com/Tencent/WeKnora/internal/application/service 3.791s`
- `go test ./internal/application/repository ./internal/application/service -count=1` — PASS: `ok github.com/Tencent/WeKnora/internal/application/repository 586.873s`; `ok github.com/Tencent/WeKnora/internal/application/service 408.166s`
- `git diff --check` — PASS (no output)
- `go build ./...` — PASS (exit 0); output warnings: `# github.com/Tencent/WeKnora/cmd/server` / `ld: warning: ignoring duplicate libraries: '-lc++'`; `# github.com/Tencent/WeKnora/cmd/desktop` / `ld: warning: ignoring duplicate libraries: '-lc++'`

No task/run/error-source functionality or interface changes were added.

R1 local commit: `8ddee80892c278b85c4ff5d5e6de5c26c640734b` (three owned source/test files only). Worktree clean after commit.
