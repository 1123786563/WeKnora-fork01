# Task 2R1 final independent re-review

Reviewed source `9b20522cb4ba3c985a4b97f9a0f3b564524bc179` at report HEAD `92b363623e70c86c90d4ecfdaba49b5091c6e0c3` against the approved Career spec, main-port architecture spec, ADRs 0015–0019, `CONTEXT.md`, Task 2R1 brief and remediation plan, prior T2/T2R1 reviews, implementation report, and fresh backend revalidation. The source is the direct parent of report HEAD. This was read-only source review; no OCR was invoked.

## Findings

### Medium — PostgreSQL fallback assertions do not bind required declarations to their tables

- **Evidence / affected symbol:** `TestCareerPostgresMigrationDeclaresIsolationAndAppendOnlyShape` in `internal/database/migration_sqlite_versioned_schema_test.go:193-204` checks three JSONB column declarations and three composite FKs across the entire SQL string. The `response_json` and `payload` checks at 193–197 also search globally. A regression that moves or duplicates one declaration while removing another can preserve those counts and pass despite a receipt, fact or evidence table losing its intended JSONB column or tenant/owner FK. The append-only trigger check does name `career_evidence` and is table-bound; the issue is specifically the JSONB/FK assertions. The current `000203_career_foundation.up.sql` has the correct table placements by manual inspection.
- **Impact:** The required static fallback can pass on a schema that fails Career owner isolation or JSONB persistence. This matters because live PostgreSQL up/down, FK enforcement, trigger behavior and GORM JSONB roundtrip remain unverified: the revalidator reached the running container but `psql` returned `the database system is in recovery mode`.
- **Smallest correction:** Parse each `CREATE TABLE` body, then assert `career_idempotency_receipts.response_json JSONB` plus its composite FK, `career_profile_facts.payload JSONB` plus its composite FK, and `career_evidence.payload JSONB` plus its composite FK. Keep the existing trigger/function and down checks. Run live PostgreSQL migration and GORM tests when the service accepts queries.

### Low — Sub-millisecond issue times can shorten an accepted grant by less than 1 ms

- **Evidence / affected symbol:** `NewVersionArtifactGrant` uses `now.Add(ttl).UnixMilli()` (`internal/modules/workbench/artifact_signing.go:71`), which truncates a fractional millisecond. `VerifyVersionArtifactGrantAt` compares `now.UnixMilli()` at line 94. For a grant issued at `.9999 ms` within a millisecond, the stored expiry reaches its integer millisecond boundary about `0.9999 ms` before the requested `ttl`. `TestNewVersionArtifactGrantPreservesOneSecondAtFractionalNow` uses `999_000_000` nanoseconds, exactly millisecond-aligned, and cannot detect that edge.
- **Impact:** A one-second or fractional-over-one-second grant may become unusable up to just under 1 ms early. The prior nearly-one-second truncation defect is corrected; there is no observed authority extension.
- **Smallest correction:** Define the wire precision explicitly and either round the computed expiry upward to the next millisecond, or store/verify Unix nanoseconds. Add a test with a non-millisecond-aligned issue time and a fractional accepted TTL.

## Closed prior findings and evidence

- **Caller authorization:** `ScopeFromContext` requires an explicit authenticated `Caller`, matching execution tenant, legacy user and `PrincipalWebUser`, and returns scope from the Caller (`internal/modules/career/repository/scope.go:11-20`). The foreign `WithExecutionTenant` and missing/mismatched Caller tests pass. No remaining tenant switch bypass was found.
- **TTL input and units:** zero, negative and positive subsecond TTLs are rejected; values over 15 minutes are capped (`artifact_signing.go:64-73`). Both creation and verification now use Unix **milliseconds**, and the fractional-second test at `.999 s` demonstrates the intended 1000 ms lifetime at millisecond alignment. Legacy Task `ArtifactGrant` remains Unix seconds and its code was not changed.
- **Repository and migrations:** the migrated SQLite GORM test roundtrips facts, receipt response and evidence payload, denies cross-owner/tenant reads, rejects an orphan fact through the composite FK and evidence update/delete through triggers, then applies Career down and checks table removal. Migration pairing and version uniqueness tests pass. The current PostgreSQL migration text manually has the expected table-specific JSONB/FK declarations and evidence trigger, but PostgreSQL runtime behavior is still unknown.
- **Scope boundary:** Task 2R2 owns the #142 public issuance/download catalog and route; Task 3 owns #141 confirmation/revision. Neither is claimed complete by this R1 review.

## Checks

| Exact check | Result |
| --- | --- |
| `git status --short && git rev-parse HEAD && git show -s --format='%P' HEAD` | Clean before this report; HEAD `92b363623e70c86c90d4ecfdaba49b5091c6e0c3`, direct parent `9b20522cb4ba3c985a4b97f9a0f3b564524bc179`. |
| `git diff --check 69f000578d9f75f141dbe0ea75790fc173502d87..92b363623e70c86c90d4ecfdaba49b5091c6e0c3` | PASS. |
| `go test ./internal/modules/career/repository ./internal/modules/workbench -run 'Test(ScopeFromContext|GormStoreSQLiteOwnerIsolationAndFoundationRoundTrip|NewVersionArtifactGrant|VersionArtifactGrant)' -count=1` | PASS; repository 4.487s, workbench 1.343s. |
| `go test ./internal/database -run 'Test(CareerPostgresMigrationDeclaresIsolationAndAppendOnlyShape|CareerMigrationPairsMatchAcrossTracks|MigrationVersionsUniquePerTrack)$' -count=1` | PASS; database 4.359s. This pass does not resolve the table-binding gap above. |
| Revalidator's `go test ./internal/modules/career/... ./internal/modules/workbench ./internal/database` | Reported PASS at the same source checkpoint; not rerun here. |
| Live PostgreSQL up/down and GORM JSONB roundtrip | Not run. Revalidator's container query failed in recovery mode; do not infer runtime compatibility from static SQL. |

## Verdict

- **Task 2R1 spec compliance: CHANGES REQUIRED.** Caller scope, TTL input/precision at millisecond alignment, SQLite persistence and migration pairing meet the brief. The explicit PostgreSQL fallback gate is still weaker than the required table-specific shape proof. Full Task 2 and product acceptance remain pending downstream tasks.
- **Code quality: CHANGES REQUIRED.** The current migration SQL looks structurally correct, and targeted runtime tests pass on SQLite. Strengthen the static regression test and retain the live PostgreSQL limitation; the sub-millisecond expiry edge is a low-severity follow-up.
