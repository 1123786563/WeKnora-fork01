# Task 2R2 C1 backend selection validation

## Verdict

**DONE_WITH_CONCERNS** — the requested regression and race checks pass at commit `64948c4b477b53d3726179d5da115a401d0ee45a`. The runtime regression proves that an immutable resource recorded on backend A remains downloadable after the tenant default changes to backend B. Tenant crossing and a scoped physical path naming a different backend are rejected. The provider mismatch branch is present and rejects a provider scheme that differs from the catalog record, but this revision has no focused runtime test for that branch.

## Evidence

- Revision: `64948c4b477b53d3726179d5da115a401d0ee45a` (`fix career artifact backend selection`); worktree was clean before checks.
- `go test ./internal/handler/session -run '^TestCareerArtifactHTTPDownloadWithProductionTenantServiceAndSingleSQLiteConnection$' -count=1` — **PASS**. The fixture sets tenant default to backend B, records the resource and version on backend A, and asserts the download body.
- `go test ./internal/application/service -run '^TestResolveResourceFileServiceRejectsTenantAndBackendScopeMismatch$' -count=1` — **PASS**. The test exercises cross-tenant rejection and a persisted physical path scoped to backend B while the resource records backend A.
- `go test -race ./internal/handler/session -run 'TestCareerArtifact(HTTPDownloadWithProductionTenantServiceAndSingleSQLiteConnection|DownloadCleansStageWhenCatalogCommitFails|HTTPDownloadReleasesSessionLockBeforeSlowResponse|HTTPIssueDownloadDigestAndRevoke)' -count=1` — **PASS**.
- `go test -race ./internal/application/service -run '^TestResolveResourceFileServiceRejectsTenantAndBackendScopeMismatch$' -count=1` — **PASS**.
- `git diff --check 64948c4b477b53d3726179d5da115a401d0ee45a^ 64948c4b477b53d3726179d5da115a401d0ee45a` — **PASS**.

## Inspection and limits

`ResolveResourceFileService` resolves the resource before download authorization, checks its tenant, compares an embedded storage backend ID with the recorded backend ID, checks the parsed provider against the recorded provider, and resolves the file service using the recorded backend ID/provider. The resulting resource-catalog wrapper resolves the reference to its stored physical path; its backend-scoped wrapper rejects a path scoped to another backend. The production handler prepares the opener before entering the catalog authorization transaction and retains the grant/version identity checks and digest/size verification in the download flow.

Provider-scheme mismatch rejection was confirmed by code inspection (`provider == "" || provider != resource.Provider`), but lacks a focused runtime test. Live PostgreSQL and external storage-provider integration were not exercised. The separate full `internal/application/service` package run is not part of this validation; focused package tests and both relevant race checks passed.
