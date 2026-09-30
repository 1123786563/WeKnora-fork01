# Task 2R2 reauthorization finding fix

**Base:** `9c3deaa56c8db765955396d616b51482c660e43d`
**Branch:** `codex/issue-140-task2r2-download`
**Scope:** R2R2-F1 only: recheck current Task ownership and deletion for Career artifact grant issuance and redemption.

## Changes

- `ArtifactCatalogStore.WithResolved` now validates the active Career binding, ready immutable version, and current `sessions.tenant_id`, `sessions.user_id`, and `sessions.deleted_at` in one database transaction. A missing, deleted, or reassigned session returns `ErrNotFound`.
- The session row is selected with `FOR UPDATE`. `BindVersion` uses the same lock while checking the publishing owner's session.
- The download handler verifies the signed grant, then invokes the catalog callback. The transaction and session lock remain open while the handler resolves storage, opens and stages the blob, verifies its size and SHA-256, and serves the response. This closes the authorization-to-blob-open race for session deletion or owner reassignment on databases honoring row locks.
- `Resolve` delegates to `WithResolved`, so issuing a new grant also rechecks the current Task owner and deletion state.
- Added repository and HTTP regression coverage: after a valid binding and successful download, changing the session owner or soft-deleting it causes both the old link and a new issuance attempt to return empty 404; the blob-open count remains unchanged.

## TDD and verification

- RED: `go test ./internal/modules/career/repository -run TestArtifactCatalogSQLiteBindsExactReadyVersionAndRechecksRevocation -count=1` failed because `Resolve` accepted the grant after changing `sessions.user_id`.
- GREEN: the same targeted repository test passed after the live session check was added.
- `go test ./internal/modules/career/repository ./internal/handler/session -run 'TestArtifactCatalogSQLiteBindsExactReadyVersionAndRechecksRevocation|TestCareerArtifactHTTPIssueDownloadDigestAndRevoke' -count=1` — passed.
- `go test ./internal/modules/career/... -count=1` — passed.
- `go test ./internal/handler/session -count=1` — passed (`48.695s`).
- `git diff --check` — passed.

## Boundaries and evidence limits

- SQLite/GORM exercised authorization behavior but SQLite does not implement PostgreSQL row-level `FOR UPDATE`; no live PostgreSQL concurrency test was run. The callback lifetime is held through response serving in code, while production row-lock behavior should still be covered by PostgreSQL integration evidence when available.
- Session permission is derived from the existing `sessions` row (`tenant_id`, `user_id`, `deleted_at`). This patch adds no schema or route changes.

## Commits

- Source commit: `b131c3200eca4d72e7dc2f0743e4389fae503af3`.
- Report commit: this report is committed separately after the source commit.
