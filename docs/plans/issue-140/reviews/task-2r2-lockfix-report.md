# Task 2R2 lock lifetime fix report

**Reviewed finding source:** `.superpowers/sdd/2026-09-28-issue-140-main-port/task-2r2-final-review.md`
**Starting HEAD:** `8ffef51cc30d72af580833fd1dcd357072128ce9`

## Changes

- **R2R2-F2 (High):** Resolve tenant metadata through the production TenantService before entering `ArtifactCatalogStore.WithResolved`. The catalog transaction therefore holds SQLite's sole connection only while checking live ownership and staging/verifying the exact artifact; it no longer nests a tenant query on that connection.
- **R2R2-F3 (Medium):** Keep the session owner lock through storage object opening, bounded staging, and digest verification. `WithResolved` returns and releases the transaction before `filetransport.Serve` writes any response bytes. The already-open mode-0600 temporary file is the authorized snapshot; a later owner change/deletion cannot change its bytes. The staging budget and temp file remain held until response completion/close, independent from database locks.
- Added regression tests using the real TenantRepository/TenantService and SQLite `SetMaxOpenConns(1)`, plus a blocking HTTP writer that proves a session owner mutation completes while response delivery is stalled.

## Verification

- RED: `go test ./internal/handler/session -run TestCareerArtifactHTTPDownloadWithProductionTenantServiceAndSingleSQLiteConnection -count=1` failed before the fix with SQLite `context deadline exceeded` and HTTP 404.
- GREEN: `go test ./internal/handler/session -run 'TestCareerArtifactHTTP(DownloadWithProductionTenantServiceAndSingleSQLiteConnection|DownloadReleasesSessionLockBeforeSlowResponse|IssueDownloadDigestAndRevoke)' -count=1` — PASS.
- `go test ./internal/handler/session` — PASS (47.128s).
- `go test -race ./internal/handler/session -run 'TestCareerArtifactHTTP(DownloadWithProductionTenantServiceAndSingleSQLiteConnection|DownloadReleasesSessionLockBeforeSlowResponse|IssueDownloadDigestAndRevoke)' -count=1` — PASS.
- `go test ./internal/modules/career/... ./internal/handler/session ./internal/router ./internal/container ./internal/database` — all package results passed; first run yielded after career packages, remaining packages were run separately and passed.
- `go test ./internal/router ./internal/container ./internal/database` — PASS.
- `git diff --check` — PASS.

## Scope and remaining limits

Only the handler and its tests changed. The PostgreSQL live locking behavior remains environment-unverified, as noted by the independent review. Slow-client response time is decoupled from the authorization transaction; bounded staging size and process stage capacity continue to limit pre-response work and staged disk use.
