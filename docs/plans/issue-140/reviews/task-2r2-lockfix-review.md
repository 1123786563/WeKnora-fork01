# Task 2R2 lockfix independent spec and quality review

**Reviewed HEAD:** `4f0e798b01ddb42e13c347120dc4b0de8200da1c` against `8ffef51cc30d72af580833fd1dcd357072128ce9`. Read the approved main-port design, ADR 0017, `CONTEXT.md`, #142 snapshot, Task 2R2 brief, all earlier R2R2 reviews, lockfix implementation and validator reports, relevant production wiring and source. This was a read-only code review; OCR was not invoked.

## Verdict

- **Spec compliance: conditional, not ready to integrate.** Exact identity, signed grant, live binding and Task-owner checks remain in place. The Task 5 material publisher is still needed to make the full #142 lifecycle reachable. The production SQLite download path retains a nested database query under its sole-connection transaction, so an authorized download cannot reliably complete.
- **Code quality: changes requested.** One high-severity availability defect remains from R2R2-F2. One medium-severity cleanup defect was introduced by moving staging outside the response helper. The response-held lock finding R2R2-F3 is fixed by code structure and its regression test. Targeted handler tests and race tests passed in independent validation, but the new single-connection test bypasses the production storage resolver.

## Findings

### R2R2-L1 — High — Production storage resolution still self-deadlocks under SQLite's one connection

**Evidence:** `CareerArtifactHandler.Download` now reads the Tenant before `WithResolved`, but `stageResolved` still calls `ResolveTenantFileServiceWithFallback` and `fileService.GetFile` inside the catalog transaction (`internal/handler/session/career_artifacts.go:155-159,197-206`). The production `StorageBackendService.ResolveFileService` calls `hydrateTenantStorage` and `ResolveBackend` (`internal/application/service/storagebackend.go:325-340`), which query its own `*gorm.DB` (`:269-281,293-319`). It also returns `resourceCatalogFileService`, whose `GetFile` calls `catalog.ResolvePath` (`internal/application/service/file/resource_catalog.go:125-135`). The catalog transaction holds the only SQLite connection (`internal/container/container.go` configures `SetMaxOpenConns(1)`). The new regression test constructs `NewCareerArtifactHandler(..., files, nil)` and therefore takes the fallback path, never exercising the production resolver or catalog-backed `GetFile`.

**Impact:** On the normal production resolver path, the nested query waits for the connection held by `WithResolved`. A valid signed URL stalls until cancellation and can occupy the sole connection, delaying unrelated requests. This continues to block the #142 authorized download acceptance in SQLite deployments.

**Smallest correction:** Prepare the storage service and resolve any catalog-backed object path before entering `WithResolved`, while deferring the actual blob open until after its live owner/binding check under the session lock. If that requires a new storage API, pass a prepared no-DB opener into the callback; do not release the owner lock before opening/staging. Add a `SetMaxOpenConns(1)` HTTP test with the production `StorageBackendService`, repository and resource catalog wiring, including a scoped or resource-catalog object key.

### R2R2-L2 — Medium — A failed catalog commit leaks the verified temp file and staging reservation

**Evidence:** `stageResolved` returns an open `stagedCareerArtifact` and its filename from within `WithResolved` (`internal/handler/session/career_artifacts.go:153-159`). `WithResolved` returns the GORM transaction result, which can fail after the callback succeeds, including during commit (`internal/modules/career/repository/artifacts.go:123-147`). `Download` checks `err` and returns at `career_artifacts.go:160-163` before installing the close/remove defers at lines 167-168. There is no cleanup at the transaction-error branch. The newly introduced staging wrapper retains the process budget until `Close`.

**Impact:** A commit failure or cancellation at that point leaves a mode-0600 artifact in the temp directory and permanently consumes its declared share of the 512 MiB staging budget. Repeat failures can disable subsequent downloads until process restart and retain private artifact bytes on disk.

**Smallest correction:** As soon as `WithResolved` returns, close and remove any non-nil staged artifact on every exit, including transaction errors; install cleanup before checking `err`. Add a catalog stub or fault-injected transaction test that invokes the callback successfully, then returns an error, and verify temp removal and budget release.

## Prior findings and evidence limits

- **R2R2-1 / R2R2-2:** `BindVersion` still checks the current same-tenant, same-owner session; the 256 MiB version ceiling and 512 MiB process reservation remain present. The capacity test and race validation previously passed.
- **R2R2-F1:** Both issuance and redemption call `WithResolved`, which checks active binding, ready immutable version, live session owner and non-deletion. The existing reassignment/deletion tests passed. PostgreSQL `FOR UPDATE` behavior still lacks a live database test.
- **R2R2-F2:** Partially addressed by moving `TenantService.GetTenantByID` before `WithResolved`; the production storage resolver and resource catalog retain nested DB calls as detailed in L1.
- **R2R2-F3:** Fixed. The verified temporary file is staged while the owner lock is held; `WithResolved` returns before `filetransport.Serve` writes to the client. The slow-writer test proves a Task owner mutation completes during blocked response writing. Serving the immutable staged snapshot afterward is consistent with the authorization check before blob open.
- Independent validation at this exact HEAD passed focused handler tests and `go test -race` for the added regressions. The test wiring omits the production storage resolver, so those passes do not establish L1's path. No live PostgreSQL test or Task 5 publisher integration was available at this checkpoint.
