# Task 2R2 cleanup fix report

**Reviewed source HEAD:** `4f0e798b01ddb42e13c347120dc4b0de8200da1c`

## Findings addressed

- **R2R2-L1 (High):** Storage backend resolution and resource catalog path resolution now happen before `WithResolved`. The file service exposes `PrepareGetFile` through the production resource-catalog and backend-scoped decorators. The returned opener does only provider I/O, and is invoked inside `WithResolved`, preserving the live binding/owner lock until the exact object has been opened and staged. The transaction snapshot's object key is compared with the prepared key before opening. `resource://` is treated as a catalog reference rather than a storage provider.
- **R2R2-L2 (Medium):** Download cleanup defers are installed immediately after `WithResolved`, before checking its returned transaction/commit error. Thus a staged file is closed, removed, and its budget reservation released even when callback work succeeded but the transaction reports failure.

## Changed files

- `internal/application/service/file/resource_catalog.go`
- `internal/application/service/file/backend_scoped.go`
- `internal/handler/session/career_artifacts.go`
- `internal/handler/session/career_artifacts_test.go`

## Verification

- `go test ./internal/handler/session -run 'TestCareerArtifact(HTTPDownloadWithProductionTenantServiceAndSingleSQLiteConnection|DownloadCleansStageWhenCatalogCommitFails)' -count=1` — PASS.
- `go test ./internal/handler/session ./internal/application/service/file -count=1` — PASS (`session`, `file`).
- `git diff --check` — PASS.

The single-connection HTTP regression uses production `TenantService`, `StorageBackendService`, SQLite storage/resource repositories, the production resource-catalog file-service decorator, a `resource://` object key, and `SetMaxOpenConns(1)`. It completes the authorized download and verifies exact bytes. The commit-failure test runs the real catalog callback to stage verified bytes, then injects the outer transaction error and verifies the stage budget is returned and no newly-created staged temp file remains.

## Remaining limits

- Live PostgreSQL row-lock behavior and commit-fault injection at the SQL driver layer were not exercised; the cleanup test injects the post-callback transaction error at the catalog seam.
- Task 5 publisher integration remains a separate downstream task.
