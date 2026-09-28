# Task 2R2-F1 independent validation

- **Status:** PASS_WITH_POSTGRESQL_LIMITATION
- **Source revision:** `b131c3200eca4d72e7dc2f0743e4389fae503af3`
- **Report revision / worktree HEAD:** `8ffef51cc30d72af580833fd1dcd357072128ce9`
- **Scope:** current Task ownership and soft-delete reauthorization at issuance and redemption; blob-open ordering; transaction/row-lock protection.
- **Changes made by validator:** none. `git status --short` was clean at inspection.

## Commands and results

1. `git rev-parse HEAD && git status --short && git show --stat --oneline --decorate b131c3200eca4d72e7dc2f0743e4389fae503af3 && git show --stat --oneline 8ffef51cc30d72af580833fd1dcd357072128ce9` — PASS; HEAD equals the specified report commit; source commit is present.
2. `go test ./internal/modules/career/repository ./internal/handler/session -run 'TestArtifactCatalogSQLiteBindsExactReadyVersionAndRechecksRevocation|TestCareerArtifactHTTPIssueDownloadDigestAndRevoke' -count=1` — PASS; both packages passed.
3. `command -v psql` — no executable found; no live PostgreSQL concurrency test was run.

The source commit's implementation report also records passing `go test ./internal/modules/career/... -count=1`, `go test ./internal/handler/session -count=1`, and `git diff --check`; I did not rerun the broader package suites because the directly relevant tests passed at the exact source revision.

## Findings

- **Ownership/deletion recheck: PASS.** `ArtifactCatalogStore.WithResolved` validates the current active binding and ready version, then requires a `sessions` row matching tenant, version's session ID, grant owner, and `deleted_at IS NULL`. `Resolve` delegates to this method, so issuance rechecks live ownership as well. The HTTP regression test mutates `sessions.user_id` to a different owner and then soft-deletes the session; for each state it checks that the old link and a new issuance both return empty 404 and that the fake file service's open count remains unchanged.
- **No blob open before authorization: PASS.** `Download` verifies the signed grant, calls `WithResolved`, and only inside its authorized callback reaches `downloadResolved` and `GetFile`. The regression assertions verify zero additional opens after each owner/deletion mutation. Size and SHA-256 validation still happen before serving bytes.
- **Authorization-to-open race protection: PASS by code inspection; live DB behavior unverified.** The catalog transaction selects the session row with GORM `clause.Locking{Strength: "UPDATE"}` and retains the transaction while the callback resolves storage, opens/stages/verifies the blob, and serves it. `BindVersion` uses the same session-row lock while authorizing a bind. On PostgreSQL, this emits a row-level `FOR UPDATE` lock that serializes the callback with a concurrent owner update or soft delete. The lock is scoped to the exact session row, not the entire tenant/catalog. `Resolve` also retains the lock through its short callback. SQLite tests validate state changes between requests but do not establish row-lock concurrency semantics.
- **Acceptance gap/risk:** PostgreSQL-specific concurrency behavior (including owner reassignment/deletion blocked until callback completes) remains untested. `psql` is unavailable in this environment, and there is no PostgreSQL test DSN evidence in this validation run. The code-level lock scope and callback lifetime are consistent with the intended protection; a live PostgreSQL integration test remains a gap.

## Overall conclusion

The assigned reauthorization behavior passes at the specified revision for sequential ownership reassignment and soft deletion, including denial of both old grants and new grant issuance without opening the artifact blob. Transaction/row-lock scope appears correctly limited to the matching session row and spans blob use. Mark the source behavior verified with the explicit PostgreSQL concurrency limitation above; do not claim live PostgreSQL concurrency verification.
