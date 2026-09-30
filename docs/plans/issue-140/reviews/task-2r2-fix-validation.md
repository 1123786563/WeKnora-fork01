# Task 2R2 finding-fix validation

- Validation revision: `9c3deaa56c8db765955396d616b51482c660e43d` (`docs: record Task 2R2 finding fixes`)
- Source fix under validation: `e3739e13f04eba77adda21b5d3ea2f98f31d9fd8` (`fix(career): bind artifact sessions and bound staging`)
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-task2r2-download-3240/WeKnora-fork01`
- Scope: read-only validation of the foreign-owner session binding and pre-blob staging budget findings. No source or test files changed.

## Result

**PASS with a bounded-scope concern.** The source commit adds an ownership check tying the ready immutable artifact version's session to the supplied Career owner and tenant before creating the binding. The repository regression test proves another owner's session returns `ErrNotFound`, while the owning session can bind and the resolved grant remains owner/resource/version/digest scoped.

The download handler now caps individual declared artifacts at 256 MiB and reserves declared bytes against a mutex-protected 512 MiB process-shared budget before resolving storage/opening the blob. A full-budget request receives an empty 404 and the blob-open counter remains zero. The reservation is released with `defer`; `-race` passed the targeted contention-denial test.

## Commands and results

All commands ran at validation revision `9c3deaa56c8db765955396d616b51482c660e43d`:

- `go test ./internal/handler/session -run 'TestCareerArtifact(HTTPIssueDownloadDigestAndRevoke|DownloadRejectsCorruptBytesBeforeSuccess|DownloadRejectsWhenStageBudgetIsFullBeforeBlobOpen)$' -count=1` — **PASS**
- `go test ./internal/modules/career/repository -run 'TestArtifactCatalogSQLiteBindsExactReadyVersionAndRechecksRevocation$' -count=1` — **PASS**
- `go test -race ./internal/handler/session -run 'TestCareerArtifactDownloadRejectsWhenStageBudgetIsFullBeforeBlobOpen$' -count=1` — **PASS**

## Acceptance gaps and risks

- The capacity test models a concurrent reservation by holding the test budget before serving a second request; it verifies denial before `GetFile`, but does not launch simultaneous HTTP requests. Mutex reservation and release are race-instrumented by the targeted test, but parallel admission behavior is not directly exercised by that test.
- The stage budget is shared by handlers in one process only. Each application replica can independently use up to 512 MiB of staging capacity, so aggregate capacity scales with replica count; there is no cross-replica/global quota.
- The reservation accounts for `version.Size`, while staging reads up to `version.Size + 1` to detect oversized blobs. A corrupt oversized stream may write one extra byte per active request before rejection, so strict physical temporary-file usage can exceed the nominal budget by the number of concurrently admitted requests. This is bounded by concurrency but not included in the accounting.
- No live PostgreSQL migration or multi-replica test was part of this assigned validation.
