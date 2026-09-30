# Task 2R2 Cleanup Fix Validation

- **Commit:** `ad8ed5c70f505e0b1b8ffa0e02218247592720ea`
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-task2r2-download-3240/WeKnora-fork01`
- **Status:** PASS for the assigned cleanup and production-storage-resolver acceptance.
- **Scope:** Read-only validation. No source or test files were modified.

## Evidence

- `git show --stat --oneline ad8ed5c70f505e0b1b8ffa0e02218247592720ea` — inspected the exact commit at HEAD; only the artifact download implementation, storage wrapper preparation methods, tests, and implementation report changed.
- `git diff --check ad8ed5c70f505e0b1b8ffa0e02218247592720ea^ ad8ed5c70f505e0b1b8ffa0e02218247592720ea` — PASS.
- `go test ./internal/handler/session ./internal/application/service/file ./internal/application/service ./internal/modules/career/...` — session package PASS (`47.442s`), file service PASS (cached). The aggregate command was interrupted while the broader application service package was still running; no failure was emitted for those first two packages.
- `go test ./internal/application/service -run 'Test.*(Resource|Storage)' -count=1` — PASS (`2.780s`).
- `go test ./internal/modules/career/... -count=1` — PASS (repository package; remaining packages have no test files).

## Acceptance checks

- Production tenant lookup occurs before the catalog transaction. Download resolves tenant metadata with the authenticated grant's tenant, then obtains an opener before entering `WithResolved`.
- `resource://` is correctly treated as a catalog reference, not a provider scheme. The production resolver test constructs `TenantService`, `ResourceCatalog`, and `StorageBackendServiceWithResources`, resolves a real local resource, and downloads through the handler.
- With SQLite configured via `SetMaxOpenConns(1)`, the HTTP download completes under a 2 second request deadline and returns exact bytes. The storage wrappers' `PrepareGetFile` methods perform DB-backed path resolution before the catalog transaction; the transaction callback then performs provider I/O without recursively querying the resource catalog.
- If the catalog transaction wrapper reports a commit failure after the callback staged the verified file, cleanup is already deferred: the staged temp file is closed/removed and the reserved stage budget is released. The regression test checks both properties.
- The staging reservation has an idempotent release closure (`sync.Once`), and `stagedCareerArtifact.Close` releases it after response streaming; all failures before staged file ownership return release it directly.

## Risks / limits

- The broad aggregate Go command did not finish because the application service package took longer than the validation window; relevant narrowed service tests and the complete career package tests passed.
- Live PostgreSQL execution and external provider I/O are not covered by this validation. The assigned SQLite single-connection and local resource catalog scenario is covered.
