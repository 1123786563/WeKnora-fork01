# Task 2R2 final independent spec and quality review

**Reviewed HEAD:** `8ffef51cc30d72af580833fd1dcd357072128ce9` (changes since `6075c5ecb863bda60e3af0d1db7a2c0cdcfe2fd2`). Read the approved Career spec, ADR 0017, `CONTEXT.md`, Task 2R2 brief, previous reviews, reauthorization fix report and validation, relevant source and tests. No source or requirement files were changed; OCR was not run.

## Verdict

- **Spec compliance: conditional / not ready for integration.** The R2R2-F1 ownership/deletion defect is fixed for sequential cases: issuance and redemption now check the current `sessions` owner and soft-delete state; the signed grant still binds exact tenant, owner, resource, version and digest. The Task 5 production publisher is still needed before the complete #142 material lifecycle can pass. The production SQLite download path has a blocking defect below.
- **Code quality: changes requested.** One high and one medium availability finding remain. The targeted repository and HTTP tests pass at this HEAD, but their fake tenant service does not exercise production DB connection use. PostgreSQL concurrency behavior remains untested on a live database.

## Findings

### R2R2-F2 — High — SQLite download self-deadlocks on its only DB connection

**Evidence:** `internal/modules/career/repository/artifacts.go:123-147` starts a GORM transaction and invokes `use(version)` before committing. `internal/handler/session/career_artifacts.go:145-146,162-164` executes `downloadResolved` within that callback, which calls the production tenant service's `GetTenantByID`. That service delegates to `tenantRepository.GetTenantByID`, which issues a new query on the same `*gorm.DB` (`internal/application/service/tenant.go:144-153`, `internal/application/repository/tenant.go:35-43`). Production SQLite config sets `sqlDB.SetMaxOpenConns(1)` (`internal/container/container.go:1553-1558`), so the transaction holds the sole connection while the nested query waits for it. `cmd/server/main.go:80-82` creates the HTTP server without a write timeout. The existing HTTP test uses `careerArtifactTestTenant`, a fake that does not query the DB.

**Impact:** Every otherwise valid Career artifact download on a SQLite deployment hangs until the request is cancelled; while hung it occupies the only DB connection and can stall unrelated application queries. The public signed URL makes this reachable with one valid grant.

**Smallest correction:** Resolve the tenant and any other database-backed storage metadata before entering `WithResolved`, then take the session lock and stage verified blob bytes using no other query on that DB connection. Add an HTTP test with the real DB-backed tenant service/repository under `SetMaxOpenConns(1)` and a short request deadline; a valid download must finish and match its digest.

### R2R2-F3 — Medium — Network response holds the Task row lock and DB connection for an unbounded client duration

**Evidence:** The `WithResolved` callback/transaction spans `filetransport.Serve` (`internal/modules/career/repository/artifacts.go:123-147`, `internal/handler/session/career_artifacts.go:145-146,198-202`). `Serve` calls `http.ServeContent` on the staged file (`internal/filetransport/response.go:49-51`), which writes to the client before returning. The main HTTP server sets no `WriteTimeout` (`cmd/server/main.go:80-82`). An authorized client can read slowly or stall after headers, retaining the session `FOR UPDATE` lock and database connection for the duration. The stage budget limits disk bytes, not this lock duration.

**Impact:** A replayed signed URL can delay Task owner changes or deletion and occupy database connections for arbitrary time; enough simultaneous slow downloads can exhaust the DB pool. This also makes the authorization transaction's completion depend on client network behavior.

**Smallest correction:** Keep the lock through tenant-independent blob open/staging and digest verification, then commit before streaming the already authorized temporary file. Set an explicit bounded response duration or write deadline if the authorization contract requires retaining the lock through delivery. Add a slow-response test proving Task mutation and DB connection release do not wait on the client after verified staging.

## Prior findings and verification

- **R2R2-1 and R2R2-2:** The foreign-owner publication check and 256 MiB artifact / 512 MiB process staging budget are present and remain supported by prior targeted/race validation. The release closure and temporary-file defer order are sound for the current handler path.
- **R2R2-F1:** `WithResolved` checks an active binding, ready version and current non-deleted session owned by the grant owner inside one transaction, and `Resolve` delegates to it. The added tests cover reassignment and soft deletion with 404 responses and no new blob open. PostgreSQL `FOR UPDATE` lock semantics are plausible by code inspection; they have not been verified with a live PostgreSQL concurrency test.
- `go test ./internal/modules/career/repository ./internal/handler/session -run 'TestArtifactCatalogSQLiteBindsExactReadyVersionAndRechecksRevocation|TestCareerArtifactHTTPIssueDownloadDigestAndRevoke' -count=1` passed at this HEAD.
- `git diff --check 6075c5e..8ffef51c` reports Markdown hard-break trailing spaces in two report files; no source whitespace error was found. This is documentation-only and not a behavioral blocker.
