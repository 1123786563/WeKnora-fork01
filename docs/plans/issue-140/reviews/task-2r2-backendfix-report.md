# Task 2R2 backend fix report — R2R2-C1

## Change

`resource://` Career artifact downloads now resolve the `StoredResource` before entering the artifact catalog's `WithResolved` transaction. The storage service verifies the resource tenant, checks that the physical path's storage backend agrees with the persisted `StorageBackendID`, checks the provider scheme against the persisted provider, and builds the file service for that recorded backend. The handler then prepares the opener outside the authorization transaction; the existing immutable object-key comparison and live grant authorization remain in place before bytes are opened/staged.

## TDD evidence

- RED: extended `TestCareerArtifactHTTPDownloadWithProductionTenantServiceAndSingleSQLiteConnection` to publish a resource on backend A while tenant default points at backend B. Before the fix, the signed download returned `404` (`download status=404 body=""`).
- GREEN: the same test now downloads the exact expected bytes successfully while retaining the single SQLite connection constraint.
- Added `TestResolveResourceFileServiceRejectsTenantAndBackendScopeMismatch` for cross-tenant reference rejection and a path whose embedded backend does not match the resource's persisted backend.

## Verification

- `go test ./internal/handler/session -run '^TestCareerArtifactHTTPDownloadWithProductionTenantServiceAndSingleSQLiteConnection$' -count=1` — PASS.
- `go test ./internal/application/service -run '^TestResolveResourceFileServiceRejectsTenantAndBackendScopeMismatch$' -count=1` — PASS.
- `go test -race ./internal/handler/session -run 'TestCareerArtifact(HTTPDownloadWithProductionTenantServiceAndSingleSQLiteConnection|DownloadCleansStageWhenCatalogCommitFails|HTTPDownloadReleasesSessionLockBeforeSlowResponse|HTTPIssueDownloadDigestAndRevoke)' -count=1` — PASS.
- `go test -race ./internal/application/service -run '^TestResolveResourceFileServiceRejectsTenantAndBackendScopeMismatch$' -count=1` — PASS.
- `go test ./internal/handler/session ./internal/application/service -count=1` — session package PASS; application/service package did not finish within four minutes and was interrupted. The focused application/service test and race variant both passed.
- `git diff --check` — PASS.

## Changed files

- `internal/application/service/storagebackend.go`
- `internal/application/service/storagebackend_test.go`
- `internal/handler/session/career_artifacts.go`
- `internal/handler/session/career_artifacts_test.go`

## Remaining limits

This fixes backend selection for already registered immutable resources. A full end-to-end Career material publication remains dependent on the downstream Task 5 publisher wiring `BindVersion` as recorded in the Task2R2 review. Live PostgreSQL/provider integration was not exercised in this fix.
