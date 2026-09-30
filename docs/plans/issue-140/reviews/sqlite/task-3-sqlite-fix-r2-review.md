# T03 SQLite startup fix round 2: independent re-review

**Range:** `3ac1b3adf7ffda75436a90783a712ec3d2ddd2ed..2f4fa9c28bf3537ce87c82c80bb01db490780315`. **Date:** 2026-09-24. Scoped to prior R1-F1 and regression risk. Sources: approved Career Spec, ADR-0015/0017, `CONTEXT.md`, #141 brief, T03 SQLite fix plan, original and round-one reviews, round-two implementer report, and SQLite `000112`/`000113` migrations. No OCR or source edits.

## Finding disposition

**R1-F1 (high, partial unique index accepted): resolved.** `requireSQLiteUniqueConstraint` now reads `partial` from `pragma_index_list` and skips any index with a nonzero value before comparing the ordered key columns (`internal/modules/career/office.go:254-284`). The new malformed-schema case replaces the full `career_facts` constraint with a partial unique index on the same three columns. It first inserts two duplicate rows excluded by the predicate, proving that the replacement index is insufficient, then asserts `NewOffice` rejects it (`internal/database/career_migration_test.go:91-112`). This directly tests the prior false positive. The same helper validates facts, changes, and receipts, so the filter applies to all three required composite constraints.

**No new blocking finding in this scoped diff.** The original F2 mixed-table path still fails closed, required columns and full composite indexes are checked, and PostgreSQL still takes its unchanged AutoMigrate branch. The fresh migration test now closes the original SQLite pool and opens a new GORM connection to the same file before initializing Career again (`career_migration_test.go:35-64`); it tests the restart-like state that the previous test lacked.

## Verification and remaining limits

- Independently ran `go test ./internal/database -run 'TestCareerOffice|TestCareerMigrationCreatesPersonalEvidenceSchema' -count=1`: passed. `git diff --check 3ac1b3ad..2f4fa9c28`: passed. The implementer separately reports RED for the partial-index case, GREEN for targeted tests, and passing Career/database/router suites; the historical RED run was not repeated here.
- The test runs the complete migration stream on a new file, then reopens it. It does not pause a preexisting database at `000112` before applying `000113`. This remains a test-evidence limit of the plan's explicit upgrade scenario. `000113` is an artifact version revocation migration and the reviewed code bases its SQLite decision on current Career schema shape; no incompatible Career migration change appears in this round.
- The guard checks required column presence and full composite unique indexes, but does not prove every column's nullability/type or read migration history. A deliberately malformed schema with nullable key columns could still weaken SQLite uniqueness for NULL values; the production `000112` migration declares those keys `NOT NULL`, and no reviewed change creates such a schema. This is a bounded recovery-schema risk, not a regression introduced by round two.
- PostgreSQL behavior is unchanged by inspection, not rerun in this scoped re-review. Actual server startup and authenticated Career HTTP probe remain parent integration checks.

## Verdict

**Scoped Spec compliance: pass. Code quality: pass.** The prior blocking partial-index finding is fixed and the required current SQLite constraints are tested after a fresh connection reopen. No effective critical/high/medium issue remains in this round's changed lines. The explicit paused-at-`000112` upgrade fixture and parent runtime/HTTP validation remain outstanding evidence for the broader task; this review does not stand in for them or for OCR.
