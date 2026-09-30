# Task 2R1 — Caller scope, grant TTL, and SQLite GORM evidence

## Checkpoint

- R1 starting BASE: `69f000578d9f75f141dbe0ea75790fc173502d87` (Task 2 evidence checkpoint).
- Source checkpoint: `fd683e903e540b2de160f4194f56fdf7b6b1ec80` (`fix: enforce career caller scope and artifact ttl`).
- Evidence report is committed separately after the source checkpoint.
- Task 2 remains unverified. This report covers R1 only; no Task 2R2 route/handler/container work was started.

### Independent-review corrections

- Follow-up source checkpoint: `9b20522cb4ba3c985a4b97f9a0f3b564524bc179` (`fix: preserve grant ttl precision and postgres checks`), based on review HEAD `5ad4bd0c1794112545b2ffea74d83e5fe99ecec0`.
- Version-grant `ExpiresAt` now stores Unix milliseconds; verification compares `now.UnixMilli()`. Legacy Task `ArtifactGrant` remains second-based and unchanged.
- Added a fractional-second RED test: at `now = 1_800_000_000.999s`, a 1-second grant initially measured `-1798200000998ms` under the old seconds representation; after the fix it measures exactly 1000ms.
- Added static PostgreSQL migration checks for all three JSONB response/payload columns, all three composite `(tenant_id, owner_id)` foreign keys, the append-only update/delete trigger and function, and their down migration removal. They passed. Live PostgreSQL remains untested because neither `pg_isready` nor `psql` is installed.
- Reran `go test ./internal/modules/career/... ./internal/modules/workbench ./internal/database` — PASS (database 85.132s).
- Reran `go test ./internal/database -run 'Test(CareerPostgresMigrationDeclaresIsolationAndAppendOnlyShape|CareerMigrationPairsMatchAcrossTracks|MigrationVersionsUniquePerTrack|SQLiteMigrationsCreateVersionedSchema)$' -count=1` — PASS (7.881s).
- `go test ./internal/modules/workbench -run 'Test(NewVersionArtifactGrant|VersionArtifactGrant)' -count=1` — PASS (1.335s).
- `git diff --check` and staged diff check — PASS.
- Report update is a separate evidence commit after the follow-up source checkpoint. This remains implementation evidence only; Task 2R1 still requires a fresh independent validation and review.

### Final-review corrections

- Final-review starting HEAD: `4f007c208076b725052806217ccb9ef76058928c` (plan/report update; source parent `9b20522cb4ba3c985a4b97f9a0f3b564524bc179`).
- Source checkpoint: `b5c68450dfa32324f19dd4775fdac98e0d899f63` (`fix: bind postgres schema checks and grant expiry`). The updated evidence is a separate following commit.
- PostgreSQL fallback test now extracts each `CREATE TABLE` body and checks its own declarations: receipts has `response_json JSONB` and composite scope FK; profile facts has `payload JSONB` and composite scope FK; evidence has `payload JSONB` and composite scope FK. Existing checks bind the update/delete trigger to `career_evidence` and verify down removes trigger/function.
- Added non-millisecond-aligned nanosecond RED evidence at `now = 1_800_000_000.987654321s`: the millisecond implementation reported `-1799998200987652334ns` versus a requested `1000000000ns`. Version grant v2 now canonicalizes, stores and checks Unix nanoseconds. Exact 1-second, exact 15-minute maximum and 1ns-before/exact-expiry tests pass. Legacy Task grants remain Unix seconds.
- `go test ./internal/modules/workbench -run 'Test(NewVersionArtifactGrant|VersionArtifactGrant)' -count=1` — PASS (0.715s).
- `go test ./internal/database -run TestCareerPostgresMigrationDeclaresIsolationAndAppendOnlyShape -count=1` — PASS (15.424s).
- `go test ./internal/modules/career/... ./internal/modules/workbench ./internal/database` — PASS; database package completed in 255.981s.
- `go test ./internal/database -run 'Test(CareerPostgresMigrationDeclaresIsolationAndAppendOnlyShape|CareerMigrationPairsMatchAcrossTracks|MigrationVersionsUniquePerTrack)$' -count=1` — PASS (3.594s).
- `git diff --check` and staged diff check — PASS.
- Live PostgreSQL remains unverified: this session has neither `pg_isready` nor `psql`; the final reviewer also observed the available container service in recovery mode. Static shape checks are not runtime PostgreSQL evidence.
- No Task 2R2 route, handler or container work was started, and Task 2R1 remains pending fresh independent validation/review.

## Changes

- `ScopeFromContext` now requires an explicit authenticated `types.Caller`, obtains the authorization tenant and owner from that caller, and checks execution tenant, user ID and `PrincipalWebUser` all match. A `WithExecutionTenant` switch to another tenant returns `ErrUnauthorized`; a legacy context with only TenantID/UserID is rejected.
- `NewVersionArtifactGrant` now rejects nonpositive and subsecond TTL. Positive TTL above `MaxArtifactGrantTTL` is capped; a 1-second TTL is accepted.
- `GormStore` stores JSON payloads as strings for SQLite TEXT/PostgreSQL JSONB compatibility and now has scoped receipt/evidence append/read operations in addition to profile facts.
- Added a migrated-SQLite GORM test that applies Career up SQL, round-trips fact/receipt/evidence payloads, checks other owners and tenants cannot read, proves composite foreign keys and append-only triggers, applies down SQL, and verifies Career tables are removed.

## RED evidence

Before production fixes, ran:

`go test ./internal/modules/career/repository ./internal/modules/workbench -run 'TestScopeFromContextRejectsForeignExecutionTenant|TestScopeFromContextRejectsMissingAndMismatchedCaller|TestNewVersionArtifactGrantRejectsInvalidTTL' -count=1`

The expected failures reproduced all three findings: foreign execution tenant returned scope `{TenantID:99, OwnerID:owner-a}`, missing Caller was accepted, and zero TTL was accepted.

## GREEN and verification

- `go test ./internal/modules/career/repository ./internal/modules/workbench -run 'Test(ScopeFromContext|GormStoreSQLiteOwnerIsolationAndFoundationRoundTrip|NewVersionArtifactGrant)' -count=1` — PASS.
- `go test ./internal/modules/career/... ./internal/modules/workbench ./internal/database` — PASS; database package completed in 86.470s. This ran after the main R1 changes; the last expansion to check receipt/evidence reads for both foreign owner and tenant was subsequently covered by the focused Career/workbench test below.
- `go test ./internal/modules/career/repository ./internal/modules/workbench -run 'Test(ScopeFromContext|GormStoreSQLiteOwnerIsolationAndFoundationRoundTrip|NewVersionArtifactGrant)' -count=1` — PASS after that final test expansion.
- `go test ./internal/database -run 'Test(CareerMigrationPairsMatchAcrossTracks|MigrationVersionsUniquePerTrack|SQLiteMigrationsCreateVersionedSchema)$' -count=1` — PASS (56.647s). Checks migration IDs/pairs and SQLite schema/head behavior.
- `git diff --check` and staged diff check — PASS.

PostgreSQL live migration/test was not run. Neither `pg_isready` nor `psql` is installed in this environment, so service availability and credentials could not be established. Versioned SQL was retained and reviewed structurally; no live PostgreSQL success is claimed.

## Integration and status

No route, handler or container files changed. R1 does not supply the production Career artifact catalog or public grant download route; those remain explicitly assigned to Task 2R2. No Task 2 verification or R2 completion is asserted here. Independent backend validation and code review are still required before downstream Task 2R2 release.
