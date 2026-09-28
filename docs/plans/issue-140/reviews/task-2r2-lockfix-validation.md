# Task 2R2 lockfix validation

**Status: PASS_WITH_LIMITATION**  
**Validated revision:** `4f0e798b01ddb42e13c347120dc4b0de8200da1c`  
**Parent revision:** `8ffef51cc30d72af580833fd1dcd357072128ce9`  
**Scope:** read-only validation of the SQLite single-connection deadlock and response-held owner lock fixes, including auth/revoke/open/hash ordering and race behavior. No source or test files changed.

## Evidence

- Read `.superpowers/sdd/2026-09-28-issue-140-main-port/task-2r2-final-review.md` and `docs/plans/issue-140/reviews/task-2r2-lockfix-report.md` at the validated HEAD.
- Inspected `internal/handler/session/career_artifacts.go`, `internal/modules/career/repository/artifacts.go`, `internal/filetransport/response.go`, production tenant service/repository, SQLite pool configuration, and both added regression tests.
- Production SQLite uses `SetMaxOpenConns(1)`. The handler now loads tenant metadata through the production `TenantService` before opening `WithResolved`'s catalog transaction. Its SQLite test constructs the real `TenantRepository` and `TenantService`, sets the pool to one connection, gives the request a deadline, and verifies a successful body equal to the expected artifact. This closes the previously identified same-connection nested-query deadlock path.
- `WithResolved` retains the live binding/version/session-owner check while `stageResolved` opens and stages bytes, enforcing the declared size and SHA-256 digest. It returns before `filetransport.Serve` writes response bytes. The slow-writer regression test blocks the first response write and proves the session owner update completes before the client resumes; returned bytes equal the already verified staged snapshot. The staging budget remains reserved until the staged response file is closed.
- Tenant lookup before owner authorization does not open artifact bytes; failures remain empty 404s. Signature/expiry validation precedes lookup. Live grant and session-owner checks precede storage resolution, blob open, and digest verification. Revocation continues to be checked in `WithResolved` on each redemption. Thus unauthorized or stale grants cannot open the blob; an already authorized request is linearized at its live catalog check while staging under the owner lock.
- `go test -race ./internal/handler/session -run 'TestCareerArtifactHTTP(DownloadWithProductionTenantServiceAndSingleSQLiteConnection|DownloadReleasesSessionLockBeforeSlowResponse|IssueDownloadDigestAndRevoke)' -count=1` — **PASS** (run during this validation).
- Reused same-revision implementation evidence from `docs/plans/issue-140/reviews/task-2r2-lockfix-report.md`: focused non-race regression command, `go test ./internal/handler/session`, and relevant career/session/router/container/database package suites all passed at this revision.
- `git diff --check 8ffef51cc30d72af580833fd1dcd357072128ce9..4f0e798b01ddb42e13c347120dc4b0de8200da1c` — **PASS**.

## Acceptance and limits

The SQLite single-connection path no longer nests a tenant query inside the catalog transaction, and slow response delivery no longer retains the database transaction/session owner lock. Auth, live revocation/owner checks, blob opening, bounded staging, and digest verification remain ordered safely. The assigned lockfix acceptance is met.

**PostgreSQL remains unverified against a live service.** The repository uses `FOR UPDATE` for the session owner row; the lock is held during staging and released before response streaming by code structure, but PostgreSQL concurrency behavior was not exercised here. This report does not establish full Task 2R2 material publisher integration, which remains a separate downstream Task 5 concern.
