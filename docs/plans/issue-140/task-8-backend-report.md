# T08/#146 JD Opportunity Backend Report

- **Task:** T08 / Issue #146, paste JD into a Career opportunity and immutable snapshot.
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t08-jd/WeKnora-fork01`
- **Base:** `11d90ce674fe84186797a4bcbd4bbe0a96ea9071`
- **Implementation commit:** `7a29b3336068f10d82d850e9f793db1d14a0970e`
- **Status:** implementation and targeted Go verification complete; ready for independent backend validation/review.

## Changed files

- `internal/modules/career/opportunity.go`: scoped import, idempotent opportunity receipt, evidence read, conservative extraction state, immutable raw text/hash, source observation and acquisition time.
- `internal/modules/career/opportunity_test.go`: Office and HTTP contract tests for replay/conflict, profile revision independence, raw snapshot fidelity, unknown fields, extraction failure, owner/Tenant scope, stable snapshot reopen and inert hostile JD text.
- `internal/modules/career/handler.go`, `internal/router/routes_career.go`: additive authenticated import, receipt recovery and fixed-snapshot evidence routes.
- `internal/modules/career/office.go`: opportunity models added to Career SQLite startup validation and AutoMigrate compatibility path.
- `migrations/sqlite/000115_career_opportunities.{up,down}.sql`, `migrations/versioned/000194_career_opportunities.{up,down}.sql`: opportunity, observation, snapshot and scoped receipt schema.
- `internal/database/career_migration_test.go`: schema column coverage and SQLite opportunity migration up/down/up test.

## Frozen backend wire

Import a pasted JD. The authenticated server supplies owner/Tenant scope, opportunity IDs, source kind and acquisition time. `sourceReference` is stored as user-provided metadata and is never fetched.

```http
POST /api/v1/career/opportunities/import
Content-Type: application/json

{"requestId":"paste-1","rawText":"Exact pasted JD text","sourceLabel":"Copied listing","sourceReference":"https://jobs.example/123"}
```

The successful `opportunity_imported` response contains `requestId`, `opportunityId`, `observationId`, `snapshotId`, `status` (`stored` or `needs_review`) and server `acquiredAt`. It does not duplicate JD text. Repeating the same scoped request ID and canonical intent returns the stored receipt and original IDs. Changing any input with that request ID returns HTTP 409 `idempotency_conflict`.

```http
GET /api/v1/career/opportunities/receipt?requestId=paste-1
GET /api/v1/career/opportunities/{opportunityId}?snapshotId={snapshotId}
```

The receipt lookup recovers the result after an ambiguous client/database response. Evidence returns `opportunityId`, `observationId`, `snapshotId`, byte-preserved `rawText`, SHA-256 `rawSha256`, separate `extracted` fields, `source` (`manual_paste`, optional label/reference), `acquiredAt`, and status. Each extracted field has `state: "known"|"unknown"`; unknown values omit `value`.

Example evidence response (IDs and time are illustrative):

```json
{
  "opportunityId":"opportunity-id",
  "observationId":"observation-id",
  "snapshotId":"snapshot-id",
  "rawText":"Exact pasted JD text",
  "rawSha256":"…",
  "extracted":{
    "title":{"state":"unknown"},
    "company":{"state":"unknown"},
    "location":{"state":"unknown"},
    "batch":{"state":"unknown"},
    "requirements":{"state":"unknown"}
  },
  "source":{"kind":"manual_paste","label":"Copied listing","referenceId":"https://jobs.example/123"},
  "acquiredAt":"2026-09-24T00:00:00Z",
  "status":"needs_review"
}
```

All fields currently remain explicitly unknown because no approved production extractor contract is wired. A parser error or partial extraction stores the raw snapshot and returns `needs_review`; it does not discard content or invent conditions. Profile revision and `/changes` do not change. Import and receipt persistence are one database transaction. Evidence reads require both IDs and the authenticated owner/Tenant pair. No JD text is passed to an Agent, tool, grant, approval decision or URL fetcher.

## TDD and verification evidence

- **RED:** `go test ./internal/modules/career` — failed to compile because the new `Office.ImportJD`, opportunity result types and snapshot model did not yet exist.
- **GREEN / focused suites:** `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...` — PASS for all listed packages. The database package includes `TestCareerOpportunitySQLiteMigrationUpDownUp`, which migrates SQLite to v115, rolls back to v114 while preserving the prior Career schema, reapplies v115, and confirms opportunity tables return.
- **Diff validation:** `git diff --check` and staged `git diff --cached --check` — PASS.
- **Architecture route discovery:** `go run ./tools/architectureguard --root .` — PASS, `literal=578 apiKeyRoute=69 handle=0 total=647`, `OK (0 violations)`. Three Career routes were added; architectureguard manifest/count files remain untouched for the parent integration task.
- **PostgreSQL migration:** `TRPC_TEST_POSTGRES_DSN` was unset (`postgres-unavailable`), so versioned PostgreSQL migration execution was not run. The v194 up/down SQL is present and follows the current versioned sequence.
- The Go linker emitted the existing macOS warning `ignoring duplicate libraries: '-lc++'`; the affected container package passed.

## Remaining limits

- No production JD extractor is configured, so a successful paste currently creates evidence with all extracted fields unknown and `needs_review`. This is intentional until an extraction contract is approved.
- PostgreSQL execution evidence requires a configured test DSN.
- The routes change architectureguard totals by three; the measured current total is 647. Parent integration owns any manifest or route-count documentation edits.
