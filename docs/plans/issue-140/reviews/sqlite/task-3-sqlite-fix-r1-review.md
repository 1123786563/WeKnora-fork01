# T03 SQLite startup fix round 1: independent re-review

**Range:** `21d45eb799f1d66e73504220835289f566aeb7e1..3ac1b3adf7ffda75436a90783a712ec3d2ddd2ed` in the T03 fix worktree. **Date:** 2026-09-24. Scoped to prior F1/F2 and regression risk. Read only source review; no OCR. Fact sources remain the approved Career Spec, ADR-0015/0017, `CONTEXT.md`, #141 brief, SQLite fix plan, actual `000112`/`000113` migrations, and the round-one implementer report.

## Prior findings

- **F1 (seven table names alone): partly resolved.** `NewOffice` now checks each required Career column and searches SQLite unique indexes for the three ordered composite keys (`office.go:225-279`). The current migrated schema passes; a missing column or missing fact constraint fails initialization in new tests. One false-positive case remains below.
- **F2 (partial tables trigger AutoMigrate): resolved.** `NewOffice` now distinguishes zero, some, and all seven tables (`office.go:194-216`). A mixed set returns an actionable error before AutoMigrate; `TestCareerOfficeRejectsPartialVersionedSQLiteSchema` verifies that a dropped receipt table stays absent. The zero-table compatibility path is exercised by existing `internal/modules/career/office_test.go` tests through `testOffice`, and `go test ./internal/modules/career -count=1` passed in this review.

## Remaining finding

### R1-F1 — High: a partial unique index passes the integrity gate

**Evidence / symbol:** `internal/modules/career/office.go:254-276`, `requireSQLiteUniqueConstraint`. The query filters `pragma_index_list` by `"unique" = 1` but does not exclude `partial = 1`. An index such as `CREATE UNIQUE INDEX uq_receipt_partial ON career_receipts(tenant_id,user_id,request_id) WHERE request_id <> 'skip'` returns the same ordered columns from `pragma_index_info`, so this function returns success. A SQLite probe confirmed `pragma_index_list` reports `unique=1, partial=1`, and two rows with `(1,'u','skip')` can both be inserted. The new tests only remove the fact constraint entirely; they do not exercise a partial replacement index.

**Impact:** A malformed or repaired seven-table database can pass Career startup while allowing duplicate request receipts (likewise facts or revisions if their uniqueness is partial), defeating idempotency and revision guarantees. This is the same data-integrity boundary prior F1 required the guard to enforce; it is not a failure of the untouched, correctly migrated `000112` schema.

**Smallest defensible correction:** Select only full unique indexes (`partial = 0`) before comparing their ordered key columns. Add a test that replaces one required full constraint with an otherwise matching partial unique index and asserts `NewOffice` rejects it. Consider nullable key columns separately if the guard is meant to validate arbitrary recovered schemas, because SQLite unique indexes permit repeated NULL key values.

## Verification and limits

- `go test ./internal/modules/career/... ./internal/database/... -run 'TestCareerOffice|TestCareerMigrationCreatesPersonalEvidenceSchema' -count=1` passed; the database tests in that selection cover clean migration, missing column/constraint, and partial table set. `go test ./internal/modules/career -count=1` separately passed and exercises the empty-schema AutoMigrate path through its existing fixture.
- The round-one report states RED for malformed and partial schemas, GREEN for targeted tests, package suite, and `git diff --check`; source and test behavior are consistent with those claims. The review did not reproduce the historical RED command.
- No test pauses an existing DB at SQLite `000112`, upgrades it to `000113`, reopens it with a new connection, and then initializes Career. The current test applies the whole migration stream to a new DB and calls `NewOffice` twice on one handle. This leaves the plan's explicit upgrade/restart verification incomplete, although `000113` is a separate artifact migration and no incompatible change is visible in the diff.
- PostgreSQL still executes `AutoMigrate` as before (`office.go:217-220`); no PostgreSQL code path was changed or independently run in this scoped review.

## Verdict

**Spec compliance: conditional, not approved for final integration.** The clean migrated SQLite path, partial-table fail-closed behavior, and empty-schema compatibility path are supported by code/tests. R1-F1 still permits a false assertion that required uniqueness is enforced. The explicit old-database upgrade/restart check remains missing. **Code quality: changes requested** for R1-F1; no separate regression was found in the PostgreSQL branch. This verdict covers this commit and review scope only, not the parent runtime HTTP probe or OCR.
