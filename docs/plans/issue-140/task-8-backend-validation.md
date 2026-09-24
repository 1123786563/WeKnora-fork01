# T08/#146 Backend Validation

- **Result:** DONE_WITH_CONCERNS
- **Requested implementation revision:** `7a29b3336068f10d82d850e9f793db1d14a0970e`
- **Validated checkout revision:** `84617e30aae66d8030682376c1fb02a0d2c9d122` (`Record T08 Career backend evidence`)
- **Revision relationship:** requested implementation SHA is an ancestor of checkout HEAD. `git diff --name-status 7a29b3336068f10d82d850e9f793db1d14a0970e..HEAD` reports only `A .superpowers/sdd/2026-09-24-issue-140-implementation/task-8-backend-report.md`; thus tracked production and test sources are exactly the requested revision. Initial and final `git status --short` were empty before this permitted report was written.
- **Brief/acceptance source:** `docs/plans/issue-140/task-8-backend-brief.md`, `docs/plans/issue-140/issues/issue-146.md`, implementation evidence `.../task-8-backend-report.md`.

## Evidence

Command run at validated checkout:

```text
go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...
```

**PASS** all listed packages. Output included `ok` for career, router, database, handler, handler/dto, handler/session, and container. Container linker emitted existing `ignoring duplicate libraries: '-lc++'` warning; package passed. This suite includes `TestCareerOpportunityImportReplaysImmutableJDAndDoesNotChangeProfileRevision`, `TestCareerOpportunityEvidenceIsScopedAndSnapshotReadIsStable`, `TestCareerOpportunityExtractionFailureRetainsRawEvidenceAndJDIsInert`, `TestCareerOpportunityHTTPContractAndOwnerScope`, and `TestCareerOpportunitySQLiteMigrationUpDownUp`.

The implementation report also records `go run ./tools/architectureguard --root .` PASS with literal=578, apiKeyRoute=69, handle=0, total=647, and `git diff --check` plus staged diff check PASS on the implementation commit. I inspected the report's claim against checked-in code and found the Career routes are additive. Parent integration owns the architectureguard baseline/count update; that exact-count integration gate remains pending and is not evidence of a backend API failure.

## Contract checks

- Import accepts request ID, raw text and optional source metadata. Scope is derived from authenticated request context and validated against the owner's personal Career tenant. HTTP handlers route all three operations through this scope check; Office queries also include both tenant and user IDs.
- Raw text, its SHA-256, source observation, acquisition time, extracted fields and receipt are written within one transaction. A scoped request ID + fingerprint returns the original receipt/IDs; changed intent maps to HTTP 409. Receipt lookup supports recovery after ambiguous outcomes.
- Evidence lookup requires opportunity ID and snapshot ID under the same owner/tenant scope and verifies the matching observation. It reads that fixed snapshot, so a later import cannot switch old evidence. Scope misses are returned as not found/unauthorized, avoiding cross-scope disclosure.
- Unknown fields are explicit states. Extractor failure or partial/unknown output preserves the raw evidence and reports `needs_review`. The tests verify profile revision remains unchanged and hostile instruction text performs no privileged work. The code contains no fetch/agent/tool dispatch in the import path; `sourceReference` is stored as metadata.
- SQLite up/down/up test passes and verifies earlier Career schema remains after rollback. Versioned PostgreSQL up/down SQL is present per implementation report, but PostgreSQL execution was unavailable because `TRPC_TEST_POSTGRES_DSN` was unset.

## Acceptance status and limitations

Backend acceptance criteria for immutable raw evidence, explicit unknowns, replay/conflict, scoped access, inert malicious text, and extraction-failure preservation are covered and pass. The Issue's “page can enter evidence details from conversation result” criterion is owned by the Web subtask and is outside this backend validation; no conclusion about its completion is made here. The approved brief's Postgres execution remains unverified without a DSN. The exact requested SHA is not HEAD, although source/test trees match it exactly as described above.

## Commands used

```text
git rev-parse HEAD
git status --short
rg --files .superpowers docs | rg '(146|task-8|issue-140)'
git show -s --format='%H %s' 7a29b3336068f10d82d850e9f793db1d14a0970e
git log --oneline --decorate -6
git diff --stat 7a29b3336068f10d82d850e9f793db1d14a0970e..HEAD
go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...
rg -n 'Opportunity|opportunity|ImportJD|receipt|snapshot|Tenant|Owner|malicious|instruction|SQLiteMigration' internal/modules/career/opportunity.go internal/modules/career/opportunity_test.go internal/modules/career/handler.go internal/router/routes_career.go internal/database/career_migration_test.go
git diff --check 7a29b3336068f10d82d850e9f793db1d14a0970e..HEAD
git status --short
git diff --name-status 7a29b3336068f10d82d850e9f793db1d14a0970e..HEAD
```
