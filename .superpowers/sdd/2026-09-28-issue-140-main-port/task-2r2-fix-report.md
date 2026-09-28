# Task 2R2 review-finding fixes

**Base:** `6075c5ecb863bda60e3af0d1db7a2c0cdcfe2fd2`  
**Branch:** `codex/issue-140-task2r2-download`  
**Scope:** R2R2-1 and R2R2-2 from `task-2r2-review.md` only.

## Changes

- `ArtifactCatalogStore.BindVersion` now reads the ready version inside the binding transaction and requires its session to exist in the same tenant, belong to `Scope.OwnerID` through `sessions.user_id`, and have no `deleted_at`. Missing, deleted, or foreign-owner sessions fail closed as `ErrNotFound`. The repository test first reproduced a foreign-owner session being bound, then verifies that it is rejected while a matching session can be bound.
- Career downloads now enforce a 256 MiB per-artifact ceiling and reserve the version's declared size from a process-wide 512 MiB temporary staging budget before tenant storage resolution or blob open. Capacity is checked without waiting; if a competing download holds the budget, the request receives the same empty 404 as other unavailable grants. Reservation release is deferred over all exits. A handler test holds the only available budget, redeems a valid URL, and verifies empty 404 and zero blob opens.

## TDD and verification

- RED: `go test ./internal/modules/career/repository -run TestArtifactCatalogSQLiteBindsExactReadyVersionAndRechecksRevocation -count=1` failed because `BindVersion` accepted a version whose session belonged to `owner-b`.
- GREEN: same command passed after adding the session ownership check.
- `go test ./internal/modules/career/... -count=1` — passed.
- `go test ./internal/handler/session -run 'TestCareerArtifact(HTTPIssueDownloadDigestAndRevoke|DownloadRejectsCorruptBytesBeforeSuccess|DownloadRejectsWhenStageBudgetIsFullBeforeBlobOpen)' -count=1` — passed.
- `go test ./internal/handler/session -count=1` — passed (`49.412s`).
- `git diff --check` — passed.

## Boundaries

- The staging limits are process-local: 256 MiB per artifact and 512 MiB total reserved bytes per process. They do not coordinate across replicas, and there is no per-owner fairness. When capacity is unavailable, clients see the deliberately non-enumerating empty 404 response.
- Session ownership is enforced using the existing session row's `tenant_id`, `id`, `user_id`, and `deleted_at`. No live PostgreSQL run was performed in this fix round; the repository behavior test uses SQLite/GORM.
- No migration, router, frontend, plan, ledger, or review report was changed.

## Commits

- Source commit: `e3739e13f04eba77adda21b5d3ea2f98f31d9fd8`.
- Report commit: recorded by the commit containing this file.
