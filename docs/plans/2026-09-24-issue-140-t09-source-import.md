# Issue 140 T09 Source Import Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Record URL import attempts with truthful source integrity and allow the user to append a complete JD as a new immutable snapshot.

**Architecture:** Career Office owns source policy, URL intent, observations, snapshots, and receipts. A narrow HTTP-only transport performs SSRF-safe fetches only after a server-owned source policy approves every host and redirect. Web renders typed status and never parses assistant prose or treats a URL as an Agent instruction.

**Tech Stack:** Go/Gin/GORM, controlled `httptest` source fixtures, PostgreSQL/SQLite paired migrations, React/TDesign Web.

**Spec:** `docs/plans/issue-140/issues/issue-149.md`; approved Job Search Spec §§3–4/8/10; ADR-0009/0015/0017; `docs/plans/issue-140/task-9-research.md`; `docs/plans/issue-140/task-9-architecture.md`.

## Global Constraints

- Work only in the assigned isolated Worktree and only on files owned by the active Task.
- Production source allowlist starts empty. No recruiting domain may be marked vetted without an authorized source-review record; unapproved URLs produce durable `policy_unverified` evidence and request user JD.
- Never use cookies, login sessions, Chromium fallback, CAPTCHA solving, anti-crawler bypass, or client-declared source trust.
- Preserve the exact submitted URL, attempt/acquisition time, source status, completeness, and bounded failure code. Do not leak raw upstream errors, private IP/redirect details, or credentials.
- Partial, login, blocked, missing, timed-out, and unverified observations never infer graduation, degree, or other hard conditions; all extracted hard fields remain `unknown`/`needs_review`.
- User JD append creates a new fixed snapshot and manual observation; it never rewrites or replaces the original URL observation.
- Network I/O must occur outside a database transaction. Durable request claims/receipts reconcile concurrent and unknown outcomes with the original request ID.
- Add only forward migrations numbered after the integrated T14 migrations; expected next files are versioned `000198_source_import_observations` and SQLite `000119_source_import_observations`, unless integration records a later number.
- Local commits are authorized; no push/merge/deploy/GitHub action.

## Review Focus

- An unapproved host must be recorded without any network attempt and must return `policy_unverified` with `needsUserJD=true`.
- A redirect from an approved host to an unapproved or private host must be rejected at the redirect, not retroactively trusted.
- A summary page containing degree words must still leave all hard fields unknown.
- URL failure followed by manual paste must leave both observations open and produce distinct snapshot IDs.
- Same request ID with changed URL or linkage conflicts; concurrent identical requests append only one observation.

---

### Task 1: Career Source Adapter And Durable URL Evidence

**Depends:** integrated T08 and the then-current integration HEAD after T14. **Owner/validator:** backend_implementer / backend_validator. **Files:** create `internal/modules/career/source_import.go` and `source_import_test.go`; modify `internal/modules/career/opportunity.go`, `office.go`, `handler.go`, focused handler tests; modify `internal/router/routes_career.go` and route tests; create migration `000198_source_import_observations` in both versioned and SQLite trees; extend `internal/database/career_migration_test.go`; add only a narrow Career-owned transport adapter, preferably in `internal/modules/career/source_import.go`, consuming an interface rather than `web_fetch.NewFetcher`.

**Interfaces:**

```go
type SourcePolicy interface {
    Verify(rawURL string) (ApprovedSource, error)
    VerifyRedirect(current, next string) error
}

type SourceTransport interface {
    Fetch(ctx context.Context, source ApprovedSource, rawURL string) (SourceFetchResult, error)
}

type ImportURLInput struct {
    RequestID string `json:"requestId"`
    URL       string `json:"url"`
}

type ImportURLResult struct {
    OpportunityID string    `json:"opportunityId"`
    ObservationID string    `json:"observationId"`
    SnapshotID    string    `json:"snapshotId"`
    SourceStatus  string    `json:"sourceStatus"`
    Completeness  string    `json:"completeness"`
    FailureCode   string    `json:"failureCode,omitempty"`
    SubmittedURL  string    `json:"submittedUrl"`
    AcquiredAt    time.Time `json:"acquiredAt"`
    NeedsUserJD   bool      `json:"needsUserJD"`
}
```

Frozen enums: `sourceStatus = complete | partial | login_required | blocked | not_found | timed_out | fetch_failed | policy_unverified`; `completeness = complete | incomplete | unknown`; failure codes exactly `login_required`, `access_blocked`, `not_found`, `timeout`, `source_unverified`, `unsupported_content`, `empty_content`, `response_too_large`, `network_error`, `redirect_disallowed`.

HTTP:
- `POST /api/v1/career/opportunities/import-url`
- `GET /api/v1/career/opportunities/:opportunityId/observations`
- `POST /api/v1/career/opportunities/import` accepts optional `opportunityId` and `priorObservationId` only for an owner-scoped URL observation append.

- [ ] **Step 1: RED source contract tests**

Add `source_import_test.go` with an in-memory policy/transport and controlled `httptest.Server` where useful. Test names:

```go
func TestImportURLCompleteCreatesImmutableOpportunityEvidence(t *testing.T)
func TestImportURLPolicyUnverifiedDoesNotFetchAndRequestsJD(t *testing.T)
func TestImportURLLoginSummaryMissingAndTimeoutClassifications(t *testing.T)
func TestImportURLRejectsUnapprovedOrPrivateRedirect(t *testing.T)
func TestImportURLDoesNotInferHardFieldsFromPartialText(t *testing.T)
func TestImportURLReplayConflictAndConcurrentSingleObservation(t *testing.T)
func TestManualJDAfterURLCreatesNewSnapshotAndPreservesTrace(t *testing.T)
func TestImportURLScopeAndErrorSanitization(t *testing.T)
func TestSourceObservationMigrationUpAndDown(t *testing.T)
```

Expected initial failure: types, routes, migration, and persistence do not exist.

- [ ] **Step 2: Implement migration and persistence**

Additively extend `career_opportunity_observations` with `source_status`, `completeness`, `failure_code`, `submitted_url`, `final_url`, `adapter_id`, `adapter_version`, and `observed_http_status`. Keep existing columns and rows valid; add scoped indexes and a down migration that drops only added columns/indexes. Failed observations still receive a snapshot row with empty text, SHA-256 of empty bytes, `needs_review`, and explicit unavailable status. Update `NewOffice` model validation and SQLite shape checks.

Extract or reuse a private persistence helper from T08 so manual and URL observations share immutable append semantics. Never hold a DB transaction across transport calls. Claim the scoped request before fetch, reconcile after fetch, and atomically insert observation + snapshot + terminal receipt.

- [ ] **Step 3: Implement policy, transport, and handler**

Implement empty production policy (`policy_unverified`) and injectable test policy/transport. Validate and preserve the submitted URL; canonicalize only for policy checks. Classify complete only through an adapter-specific positive completeness check. Map transport failures to bounded codes. Add owner-scoped observation list. Extend manual import with prior IDs after verifying scope/linkage and append rather than merge.

- [ ] **Step 4: Verify and commit**

```bash
gofmt -w internal/modules/career internal/router
go test ./internal/modules/career/... -count=1
go test ./internal/database/... -count=1
go test ./internal/router/... -count=1
go test ./tools/architectureguard/... -count=1
git diff --check
git add internal/modules/career internal/router migrations/versioned/000198_source_import_observations.* migrations/sqlite/000119_source_import_observations.* internal/database tools/architectureguard
git commit -m 'feat(career): record URL source evidence'
```

### Task 2: Web Source Status And Paste Fallback

**Depends:** reviewed and integrated Task 1 exact API commit. **Owner/validator:** frontend_implementer / frontend_validator. **Files:** modify `apps/web/src/career/OpportunityPage.tsx`, its tests and local CSS; `packages/api-client/src/career.ts` and focused test; route tests only if a new detail subroute is required.

**Interfaces:** consume `importUrl`, `opportunityObservations`, and extended `importOpportunity` exactly as serialized by Task 1. Preserve request ID through unknown recovery and use a new ID only for the subsequent manual paste.

- [ ] **RED:** add browser-level tests for complete URL, `policy_unverified`, login/summary/timeout, unknown POST recovery, 403/scope clear, and failure → paste → new snapshot → original trace. Assert incomplete text cannot show hard-condition conclusions.
- [ ] **GREEN:** render typed status, submitted link, attempt/acquisition time, bounded reason, and “需用户补充 JD”. Paste sends prior observation IDs and opens the new snapshot; history links remain inert and owner-scoped.
- [ ] **VERIFY:** focused tests, `pnpm typecheck:web`, `pnpm test:web`, `pnpm build:web`, `git diff --check`; then local authenticated browser fixture with controlled source responses.
- [ ] **COMMIT:** `git add apps/web/src/career packages/api-client && git commit -m 'feat(web): guide career URL import fallback'`.

## Self-Review

- Source trust, SSRF/redirect, incomplete evidence, immutable append, recovery, scope, and Web fallback all have named tests.
- Production behavior intentionally has no vetted live source; release source coverage remains a separate T33 gate.
- Migration numbering is conditional on the recorded integration HEAD and must be reconciled before dispatch.
