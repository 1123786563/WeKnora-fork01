# Task 2R1 final independent spec and quality review

Reviewed implementation `b5c68450dfa32324f19dd4775fdac98e0d899f63` at evidence HEAD `3fc8f6bbfbaa17a807b54c03310ce14d96501fe9` against the approved Career spec, main-port architecture spec, ADRs 0015–0019, `CONTEXT.md`, #141/#142 snapshots, Task 2R1 brief and remediation plan, and the prior Task 2/2R1 reviews. This review is read-only for production code and requirements. No OCR was invoked.

## Findings

No actionable critical, high, medium, or low finding in the Task 2R1 correction scope.

## Evidence

- **PostgreSQL static fallback:** `TestCareerPostgresMigrationDeclaresIsolationAndAppendOnlyShape` (`internal/database/migration_sqlite_versioned_schema_test.go:186-210`) extracts each named `CREATE TABLE` body and checks its own JSONB payload and `(tenant_id, owner_id)` foreign key. The migration text places these declarations in `career_idempotency_receipts`, `career_profile_facts`, and `career_evidence`; the test also checks the evidence trigger/function and down removal. This closes the prior global-count weakness. It is structural evidence only; it does not prove PostgreSQL accepts or enforces the SQL.
- **Grant lifetime:** `NewVersionArtifactGrant` (`internal/modules/workbench/artifact_signing.go:64-73`) rejects nonpositive and subsecond TTLs, caps only values above 15 minutes, and records `now.Add(ttl).UnixNano()`. Verification rejects at `now.UnixNano() >= ExpiresAt` (`:93-104`). The non-millisecond-aligned tests assert an exact one-second and maximum lifetime, acceptance one nanosecond before expiry, and rejection at expiry. The `wk-version-artifact-v2` canonical prefix makes the changed expiry unit an explicit new version; repository search found no production caller or issued v1 version grant to migrate.
- **Authenticated scope:** `ScopeFromContext` (`internal/modules/career/repository/scope.go:10-21`) requires an explicit Caller, matching execution tenant and legacy user, and a matching Web-user principal. The foreign `WithExecutionTenant`, missing Caller, and mismatched identity tests cover the earlier tenant switch defect.
- **SQLite persistence:** `TestGormStoreSQLiteOwnerIsolationAndFoundationRoundTrip` exercises migrated GORM fact, receipt and evidence roundtrips, other owner/tenant read denial, composite FK rejection, append-only evidence trigger rejection, and Career down migration. The independent validator reports the broader assigned package suite passing at the same source revision. The legacy `ArtifactGrant` remains a separate Unix-seconds contract, and its tests pass.

## Checks performed

| Check | Result |
| --- | --- |
| `git status --short && git rev-parse HEAD` before this report | Clean; HEAD `3fc8f6bbfbaa17a807b54c03310ce14d96501fe9`. |
| `git diff --check 9b20522cb4ba3c985a4b97f9a0f3b564524bc179..b5c68450dfa32324f19dd4775fdac98e0d899f63` | PASS. |
| `go test ./internal/database -run '^Test(CareerPostgresMigrationDeclaresIsolationAndAppendOnlyShape|CareerMigrationPairsMatchAcrossTracks|MigrationVersionsUniquePerTrack)$' -count=1` | PASS. |
| `go test ./internal/modules/career/repository ./internal/modules/workbench -run '^Test(ScopeFromContext|GormStoreSQLiteOwnerIsolationAndFoundationRoundTrip|NewVersionArtifactGrant|VersionArtifactGrant)' -count=1` | PASS. |
| Live PostgreSQL up/down and GORM JSONB roundtrip | Not run. The independent validator reports a PostgreSQL container in recovery mode and no successful query. |

## Verdict

- **Task 2R1 spec compliance: PASS for the assigned correction scope.** The Caller boundary, TTL input/expiry rules, SQLite GORM behavior, and table-scoped PostgreSQL fallback checks have supporting code and tests.
- **Code quality: PASS with a runtime evidence limit.** No remaining actionable defect was found in this checkpoint. PostgreSQL migration execution, FK/trigger enforcement, and GORM JSONB roundtrip still require live validation when the service accepts queries. Static assertions and SQLite behavior must not be reported as PostgreSQL runtime proof.
- **Broader product acceptance: pending.** Task 2R2 owns the #142 catalog-backed issuance/download routes and live revocation checks; Task 3 owns #141 confirmation/revision behavior. This R1 verdict does not mark either ticket or the full Career spec complete.
