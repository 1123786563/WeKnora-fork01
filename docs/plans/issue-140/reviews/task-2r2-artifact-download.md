# Task 2R2 — Career Artifact grants and versioned downloads

## Scope and commits

- Assigned BASE: `ed6cf08a3ef6986322bc01681dc797e086efb180` (`codex/issue-140-task2r2-download`).
- Source commit: `19097b78452ba9309f755f89c439e79f3453c5ca` (`feat: wire career artifact grants and downloads`).
- Source test follow-up: `4a56c004f36ce7d873aeb60ecc402cab6503e60d` (`test: cover career grant router registration`).
- Source HEAD reviewed by these commands: `4a56c004f36ce7d873aeb60ecc402cab6503e60d`.
- This evidence file is committed separately from the source commits; its resulting SHA is reported in the task handoff.
- Task 2R2 remains pending independent backend validation and review. This report does not mark the task verified.

## Behavior and integration

- Added durable Career-to-Artifact bindings in `career_artifact_bindings`, with a composite `(tenant_id, owner_id, resource_id, version_id)` identity, a scoped foreign key to `career_spaces`, a tenant/version foreign key to immutable `artifact_versions`, and explicit active/revoked/deleted states. New paired migration IDs are SQLite `000125` and versioned/PostgreSQL `000204`; prior heads were SQLite `000124` and versioned `000203`.
- `ArtifactCatalogStore.BindVersion` is the trusted publisher hook. It only accepts a server-derived Career `Scope` and resource/version IDs, resolves a ready ArtifactVersion, and reads digest/object key/MIME/size from that row. `Resolve` requires the exact active tenant/owner/resource/version binding and exact ready-version digest. Revoke and logical delete immediately make subsequent authorization fail.
- Authenticated issuance is `POST /api/v1/career/resources/:resource_id/versions/:version_id/signed-url`. It uses `repository.ScopeFromContext`, resolves the digest server-side, and issues a five-minute Workbench nanosecond-expiry grant. The response URL includes the signed identity/version/digest fields; no client-supplied owner, tenant, digest, object key or expiry is accepted for issuance.
- Public redemption is `GET /api/v1/career/artifacts/download`, registered before global Auth. It verifies the HMAC and expiry, then calls the Career authorization adapter on every request and resolves the exact version again before `GetFile`. Denials return an empty 404. Blob data is staged in a mode-0600 temporary file, bounded to the declared size plus one byte, and SHA-256/size checked before any successful HTTP response is written. Request cancellation is checked during staging.
- DI is `container.NewCareerArtifactHandler`; `RouterParams.CareerArtifactHandler` is optional and `NewRouter` calls `RegisterCareerArtifactDownloadRoute` before Auth plus `RegisterCareerArtifactRoutes` within the authenticated v1 group. Legacy `/api/v1/workbench/artifacts/download` and W26 session artifact-version download code were not changed.
- Career persistence uses its own `ArtifactGrant` query shape. The session handler adapts the Workbench signing authority to Career's catalog port, avoiding a cross-module import from Career repository code.
- Existing application code has no Career resource publisher that calls `BindVersion` yet. Future resource publication must call this trusted method after its immutable version is ready; until a publisher exists, only pre-bound resources can receive URLs. No Task 5 material artifact was available here.

## Verification

Commands run from the assigned worktree:

- `go test ./internal/modules/career/... ./internal/modules/workbench ./internal/handler/session ./internal/router ./internal/container ./internal/database` — PASS. This includes the GORM/SQLite binding catalog test, full fresh SQLite migration application, handler/session tests, router tests, DI/container tests, and database migration tests.
- `go test ./internal/handler/session -run 'Test(DownloadWorkbenchArtifactGrant|DownloadArtifactVersion|CareerArtifact)' -count=1` — PASS. The HTTP test issues a URL, downloads fixture bytes `actual immutable artifact bytes`, and verifies SHA-256 `e7e05ab7821f25050a17dec29835ac4edebdfca2673d9f38defe299df1213b4d`; validly signed foreign tenant/owner/resource/version/digest claims, expired/tampered grants, revoked/deleted state, and corrupt bytes are denied without an unauthorized blob open or success body. Existing Task grant and W26 version-download tests pass.
- `go test ./internal/router -run 'TestRegisterCareerArtifactRoutesExposesOnlyIssueOnAuthenticatedGroup|TestFrontendStaticDoesNotInterceptResourceGrant' -count=1` — PASS. Covers both registered Career paths and the existing frontend static/resource-grant routing guard.
- `git diff --check` — PASS before source commit; staged diff check also passed before source commit.
- `make check-backend-architecture` — exits 1 on both this HEAD and the exact BASE. BASE reports 613 literal routes / 700 total and a missing placeholder Career route entry; this HEAD reports 615 / 702, removes the missing Career route entry, and adds the two owned Career route functions. Remaining forbidden imports, unowned horizontal files and route gaps match the BASE diagnostics; no new Career-specific architecture finding remains.
- PostgreSQL live migration execution was not available/performed. The versioned migration is paired and textually checked for the composite PK/FKs/index; live PostgreSQL application remains unverified.

## Remaining validation points

- Independent review should confirm the authorization and temporary-file streaming boundary, and controller validation should exercise actual `NewRouter` assembly with the production storage resolver.
- `BindVersion` currently has no non-test caller because the Career resource publisher is assigned downstream. Keep this explicit as an integration dependency; do not claim material Career URL issuance is live until that publisher writes the binding.
- PostgreSQL migration application was not exercised against a live service. Task 2R2 is not verified until independent validation/review accepts these limits and all required findings are resolved.
