# Task 2R2 Independent Backend Validation

- **Verdict:** PASS WITH CONCERNS (task scoped to catalog binding + signed grant issue/download); Task2R2 remains conditional on Task5 wiring for live material publication.
- **Revision:** `6075c5ecb863bda60e3af0d1db7a2c0cdcfe2fd2`
- **Assigned BASE:** `ed6cf08a3ef6986322bc01681dc797e086efb180`
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-task2r2-download-3240/WeKnora-fork01`
- **Inputs:** `docs/plans/issue-140/reviews/task-2r2-artifact-download.md`; Task2R2 brief was referenced by the parent assignment but is absent from this worktree. Scope assessed from that assignment and the implementation report.
- **Working tree:** clean before this validation report was created.

## Commands and results

All commands ran at the exact revision above:

1. `go test ./internal/handler/session -run 'Test(DownloadWorkbenchArtifactGrant|DownloadArtifactVersion|CareerArtifact)' -count=1` — **PASS** (`ok`, 3.950s).
2. `go test ./internal/router -run 'TestRegisterCareerArtifactRoutesExposesOnlyIssueOnAuthenticatedGroup|TestFrontendStaticDoesNotInterceptResourceGrant' -count=1` — **PASS** (`ok`, 2.710s).
3. `go test ./internal/modules/career/... ./internal/container ./internal/database` — **PASS** (Career repository, DI/container and migration packages; one linker warning about duplicate `-lc++` only).
4. `git diff --check ed6cf08a3ef6986322bc01681dc797e086efb180..6075c5ecb863bda60e3af0d1db7a2c0cdcfe2fd2` — **PASS**.

The implementation report additionally records the broader same-HEAD command `go test ./internal/modules/career/... ./internal/modules/workbench ./internal/handler/session ./internal/router ./internal/container ./internal/database` as PASS. Targeted commands above independently re-ran the critical handler, route, catalog, migration, and DI checks.

## Acceptance evidence

- **Authenticated issuance scope:** `CareerArtifactHandler.Issue` derives tenant and owner using `repository.ScopeFromContext`; the helper enforces authenticated Caller == execution tenant/user == web principal. Issuance takes resource/version from route params and resolves the digest from the ready immutable version row. No client-provided digest, object key, owner, tenant, or expiry is accepted.
- **Download authority:** `Download` rejects invalid/missing HMAC capability with an empty 404; `VersionArtifactGrantAuthority.Authorize` verifies signature/expiry and invokes the live catalog authorizer on every request. The handler then resolves the exact tenant/owner/resource/version/digest binding again and verifies all returned identity fields before opening storage.
- **No blob read before authorization:** code order is signature/live authorization → exact catalog resolution → tenant/storage resolution → `GetFile`. Handler tests assert denied foreign, expired/tampered, revoked/deleted and unknown grants do not open unauthorized blobs and emit no success body.
- **Byte integrity and error behavior:** bounded read is `declared size + 1`, streamed into a mode-0600 temporary file while hashing; size or SHA-256 mismatch returns 404 before response success. Reader honors request cancellation between reads. Storage, tenant, key, catalog, and file errors fail closed as non-enumerating 404; missing signing key on issuance returns 501.
- **Catalog consistency:** `BindVersion` accepts server-side `Scope` and IDs only, reads object key/digest/MIME/size/session from the ready artifact version, and transactionally creates Career-space/binding rows. `Resolve` requires exact active binding and ready version/digest. Revocation and logical deletion cause subsequent authorization to fail. SQLite GORM tests cover tenant/owner/resource/digest isolation, ready binding, revocation/deletion and down migration.
- **Schema:** SQLite `000125` and versioned `000204` migrations add a composite binding key, scoped FK to `career_spaces`, tenant/version FK to immutable `artifact_versions`, and lookup index. SQLite migrations are exercised by tests. PostgreSQL migration application was not run against a live database.
- **Production wiring:** `container.NewCareerArtifactHandler` constructs the repository catalog using the application `*gorm.DB` and injects tenant/file/storage services; it is provided in the DI container. `NewRouter` mounts the public signed GET before global Auth and registers POST on the authenticated v1 group. Route tests verify both paths and legacy/static route guard. The legacy Workbench and W26 handlers are untouched by this diff.

## Acceptance gaps and risks

1. **Task5 publisher dependency:** there is no non-test caller of `ArtifactCatalogStore.BindVersion` at this revision. Therefore no Career material can obtain a grant from the production flow until Task5 (or another authorized publisher) binds its ready immutable version. This is an explicit downstream seam in the implementation report; Task2R2 alone proves the lifecycle only for already-bound catalog entries. Do not claim end-to-end Career material publication/download is live until Task5 is integrated and tested.
2. **Production router assembly not exercised end-to-end:** route registration functions and DI provider are separately tested/inspected, but there is no test instantiating the full production `NewRouter` with the resolved handler and invoking both routes. No defect was observed in wiring, but a missing/incomplete `RouterParams` injection could leave endpoints unmounted in a particular assembly. Controller validation requested this specifically; treat as residual integration risk.
3. **Live PostgreSQL unavailable:** versioned migration has matching shape by inspection, but actual PostgreSQL DDL/FK behavior, transaction semantics and GORM queries have not been exercised against PostgreSQL. SQLite evidence does not eliminate dialect risk.
4. **Cancellation is cooperative:** `contextReader` checks context before each underlying `Read`; if a storage reader blocks indefinitely without honoring context, this wrapper cannot interrupt that blocked call. The existing storage/file service cancellation contract was not verified here.

No blocking security or authorization failure was found in assigned Task2R2 scope. Because the production publisher and full router assembly are downstream/unverified, report status is **PASS WITH CONCERNS**, not unconditional end-to-end acceptance.
