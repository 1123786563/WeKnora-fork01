# Task 2 — Career module, schema and Artifact grant evidence

## Scope and commits

- Assigned BASE: `715f4a04adbc5ddf637dd34988e3b4c1edc4a900` (`codex/issue-140-task2-module`).
- Source checkpoint: `342dee842b8e825d9a84852fc47c13db1d4befec` (`feat: establish career module and artifact grants`).
- This report is a separate evidence commit following the source checkpoint. Task remains pending independent validation; this report does not mark it verified.
- No central router, container or migration runner registration was changed.

## Files and behavior

- Added `internal/modules/career/**`: module ownership manifest, documented module boundary, scope-derived Identity adapter, GORM-backed owner-scoped confirmed-fact repository base, service seam, handler package placeholder and owner/tenant boundary tests.
- Added paired migrations: versioned `000207_career_foundation` and SQLite `000128_career_foundation`. Both create composite `(tenant_id, owner_id)` space keys, scoped idempotency receipts and profile facts, plus owner-scoped evidence. Evidence UPDATE/DELETE is rejected by database triggers. IDs were free after scanning both migration tracks; existing uniqueness test covers duplicate IDs.
- Added Workbench `VersionArtifactGrant` authority alongside the legacy `ArtifactGrant`. Its HMAC binds tenant, owner, resource, exact version, 64-hex SHA-256 digest and expiry. Server-side issuance caps TTL at `MaxArtifactGrantTTL`; the authority calls `VersionArtifactGrantAuthorizer` during issuance and each download authorization. Existing Task/session/message grant fields and endpoints remain unchanged.
- Added Career ownership/migration entries in `backend-modules.yaml` and strict-schema `moves/career.yaml`. `moves/README.md` clarifies the new manifest is supplemental to the existing Pass A set.

## Verification

Commands run from the assigned worktree:

- `go test ./internal/modules/career/... ./internal/modules/workbench ./internal/database` — PASS. Career repository/service packages compiled; owner/scope, exact-version grant, tamper/expiry/digest/revocation checks passed; full database package passed, including fresh SQLite migration schema/head and evidence append-only behavior (`internal/database` completed in 122.753s).
- `go test ./internal/database -run 'Test(CareerMigrationPairsMatchAcrossTracks|MigrationVersionsUniquePerTrack|SQLiteMigrationsCreateVersionedSchema)$' -count=1` — PASS (25.899s). Confirms IDs 203/124 are unique, up/down create/drop table sets match, current SQLite migration head applies and Career tables/triggers exist.
- `go test ./internal/modules/workbench -count=1` — PASS.
- `go test ./internal/handler/session -run 'Test(DownloadWorkbenchArtifactGrant|DownloadArtifactVersion)' -count=1` — PASS before the final additive authority wrapper; it covers the unchanged public Task grant and version download handlers. A final rerun was started but stopped after 3 minutes with no output while package compilation stalled; no final rerun result is claimed.
- `go test ./internal/router -run TestFrontendStaticDoesNotInterceptResourceGrant -count=1` — PASS before final additive changes. The chained final rerun did not reach router after the session package compilation stall.
- `git diff --check` — PASS before source commit (`342dee8`).
- `make check-backend-architecture` — FAILS on existing repository-wide boundary findings after successfully strict-decoding 17 manifests. Reported diagnostics include pre-existing codedelivery/workbench forbidden imports, unowned horizontal repository files and route coverage gaps; no Career strict-YAML error was reported. These findings are outside Task 2 ownership and were not modified.

## Controller integration hooks

1. Derive identity with `repository.ScopeFromContext(ctx)`, which requires authenticated `TenantID`, `UserID` and matching `Principal`; ignore all client-supplied tenant/owner authority.
2. In the authenticated Career route/container composition, construct `VersionArtifactGrantAuthority{Secret: <configured signing key>, Authorizer: <Career resource catalog adapter>}`. Use `Issue` only after resolving the resource and exact immutable version server-side.
3. Implement `VersionArtifactGrantAuthorizer.AuthorizeVersionGrant(ctx, grant)` against the Career resource-to-owner binding and Workbench Artifact version catalog. It must check tenant, owner, resource, exact version, digest, published/readable state, deletion and revocation on each call. Existing `artifact_versions` rows are tenant/session scoped and do not themselves establish Career owner authority.
4. Add the authenticated grant-issuance route and public grant-download route in `internal/router`/session HTTP integration, then register dependencies in `internal/container`. On every download call `authority.Authorize` before opening the stored blob; return a non-enumerating not-found/unauthorized response on failure. Preserve `/api/v1/workbench/artifacts/download` Task URLs.
5. Migration runner already loads versioned and SQLite files from their respective migration directories in numeric order; no hand-maintained central migration registration was found. Integration owner should add module/container route wiring only.

## Remaining risks / limits

- The grant lifecycle is a reusable public authority, not an HTTP endpoint. Actual download-time recheck and a real browser/Web download digest require the integration owner to implement the resource catalog adapter and route described above.
- No PostgreSQL service was available/run here; the PostgreSQL migration was reviewed structurally, while SQLite migration execution and down-pair table-set checks passed. The GORM repository targets the composite schema; end-to-end GORM CRUD was not covered by a database-backed module test.
- Architecture guard currently exits nonzero on unrelated existing boundary diagnostics. Independent validator should compare those findings with the BASE before deciding whether any are newly introduced.
- Do not mark Task 2 verified until the independently reviewed integration checkpoint supplies the owner-resource adapter, download-time auth/revocation path, migration runner evidence and requested real download digest.
