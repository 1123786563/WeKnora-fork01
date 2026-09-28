# Task 2R2 independent final review

**Scope:** `ed6cf08a3ef6986322bc01681dc797e086efb180..64948c4b477b53d3726179d5da115a401d0ee45a`, clean worktree at review start. I read #142, the Task 2R2 brief, the approved Career facts/ADR 0017, all prior Task 2R2 findings and fix/validation reports, and the changed handler, catalog, storage resolution, routes, migrations and tests. Read-only review; no OCR run.

## Verdict

- **Task 2R2 spec compliance: PASS for an already published immutable version.** Authenticated issuance derives owner and tenant from the caller; the signed grant fixes tenant, owner, resource, version, digest and expiry. Redemption verifies the signature and live binding, ready version and current Task ownership/deletion before blob open, then checks the exact byte count and SHA-256 before returning success. Denials have the same empty 404 shape. The public route and authenticated issuance route are registered, and prior validation passed legacy Task/W26 download tests.
- **Code quality: PASS with evidence limits.** All prior high/medium findings, including R2R2-C1, are closed at this HEAD. The final backend fix selects the stored resource's recorded backend and provider before entering the authorization transaction. The new production-path test proves download from backend A after the tenant default has moved to backend B with one SQLite connection. No new blocking issue was found in this complete diff.
- **Overall #142 lifecycle remains conditional on Task 5.** No non-test Career material publisher calls `BindVersion` yet. Task 5 must bind its generated version and verify a real material download before the complete user flow can be accepted. Live PostgreSQL and external-provider integration also remain untested; these are evidence gaps, not demonstrated defects in this review.

## Prior finding closure

| Finding | Status and evidence |
| --- | --- |
| R2R2-1 foreign Task publication | Closed. `BindVersion` reads the ready version and requires its current session to match tenant and owner before inserting the binding; repository regression covers a foreign owner's session. |
| R2R2-2 unbounded temporary staging | Closed. Versions above 256 MiB are denied and a mutex-protected 512 MiB process budget is reserved before blob open, with idempotent release. Capacity-denial and race tests passed. |
| R2R2-F1 stale Task owner/deletion | Closed. Both issuance and redemption use `WithResolved`, which checks active binding, ready version and live non-deleted session owner; HTTP tests deny old grants and new issuance after reassignment or soft deletion without another blob open. |
| R2R2-F2 / R2R2-L1 single-connection SQLite deadlock | Closed. Tenant, version, storage backend and resource path resolution occur before `WithResolved`; inside its callback the prepared opener performs provider I/O and staging. The production storage/resolver HTTP regression passes with `SetMaxOpenConns(1)`. |
| R2R2-F3 lock held during client response | Closed. Verified bytes are staged under the session lock, then `WithResolved` returns before `filetransport.Serve`; a slow-writer test shows Task mutation can proceed during response writing. |
| R2R2-L2 failed commit leaks temp file/budget | Closed. Close/remove defers are installed for a nonnil staged artifact before handling the transaction error. Fault-injected post-callback failure verifies cleanup and budget release. |
| R2R2-C1 current default backend used for old resource | Closed. `ResolveResourceFileService` checks the resource tenant, scoped backend ID and provider provenance, then resolves the recorded backend ID/provider. The final HTTP regression publishes on A, sets the tenant default to B and downloads exact bytes. A focused service test rejects cross-tenant and backend-scope mismatch. |

## Verification and limits

- I ran `go test ./internal/handler/session -run 'TestCareerArtifact' -count=1`: PASS at `64948c4b`.
- The independent backend validator ran the backend-A/default-B regression, cross-tenant/backend mismatch test and relevant handler/service race tests at this exact HEAD: PASS. The provider-scheme mismatch branch is code-inspected but lacks a focused runtime test.
- Earlier independent validation passed the targeted legacy Workbench and W26 download tests, router registration/static routing, Career catalog/container/database packages and the broad Task 2R2 Go package suite at their respective checkpoints. The final fix changes storage selection and the Career handler only; it does not alter the legacy routes.
- `git diff --check ed6cf08a..64948c4b` reports four trailing-space lines in two older `.superpowers/sdd/` Markdown evidence reports. This is **low severity**, documentation formatting only. The smallest correction is to remove those hard-break spaces before final branch integration if a clean full-range diff gate is required.
- No live PostgreSQL row-lock/migration run, external storage provider run, or full production `NewRouter` assembly request was performed. The Task 5 publisher and its material-to-version binding are outstanding downstream acceptance work.
