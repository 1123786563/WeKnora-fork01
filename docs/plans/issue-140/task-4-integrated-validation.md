# T04 / Issue #142 Integrated Independent Validation

- Status: `DONE_WITH_CONCERNS`
- Validated revision: `2e50f6b1eb8d7c7e2c0bde7a338336e985ec97d0`
- Validation worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`
- Acceptance brief: `docs/plans/issue-140/issues/issue-142.md` (GitHub REST snapshot, 2026-09-24 Asia/Shanghai)
- Implementation plan: `docs/plans/2026-09-21-lago-t04-pricing-group.md` (unrelated older plan; retained as supplied context)
- Prior same-scope validation: `task-4-validation.md` at `7e74116ae3381c6c0b80290f1109d016ed1fb9d7`
- Prior digest-gate feasibility: `task-4-digest-gate-research.md` at the same base revision
- Working tree: clean before report creation; validation did not modify source or tests.

## Exact-revision checks

All commands below ran in the integrated worktree at the revision above:

1. `go test ./internal/handler/session -run 'Test(CreateWorkbenchArtifactVersionSignedURLBindsFixedReadyVersion|DownloadArtifactVersionGrantRechecksOwnerAndRevocation|DownloadArtifactVersionGrantRequiresCurrentActiveMembership|DownloadArtifactVersionGrantStreamsExactBytesAndRevokedIssuedLinkFails|DownloadWorkbenchArtifactGrantRejectsTamperedAndExpired|DownloadWorkbenchArtifactGrantFailsClosedWithoutKey|DownloadWorkbenchArtifactGrantResolvesAndStreams)$' -count=1` — PASS (`ok`, 0.870s).
2. `go test ./internal/application/repository -run 'TestArtifactVersion|TestArtifactVersionsMigration' -count=1` — PASS (`ok`, 7.317s). This package's formal migration test applies the SQLite stream in a fresh temporary database and checks artifact version table creation/down behavior.
3. `go test ./internal/router -run 'TestWorkbenchArtifactVersionRevocationRouteMounted|TestArtifactVersionDownloadRoute' -count=1` — PASS (`ok`, 0.988s).
4. `go test ./internal/container -run '^TestArtifactVersionDownloadHandlerWired$' -count=1` — PASS (`ok`, 1.844s). Linker emitted the existing duplicate `-lc++` warning.
5. `git rev-parse HEAD` — `2e50f6b1eb8d7c7e2c0bde7a338336e985ec97d0`.

## Migration and contract review

- Inspected `migrations/versioned/000191_career_profile.up.sql`, `migrations/versioned/000192_artifact_version_revocation.up.sql`, and SQLite counterparts `000112`/`000113`. PostgreSQL `000192` adds `revoked BOOLEAN NOT NULL DEFAULT FALSE`; its down migration drops only that column. SQLite `000113` has the same up/down intent. This preserves existing rows as unrevoked by default.
- Isolated SQLite DDL sequence was exercised with the actual files against `/tmp/weknora-t04-migrate-integrated-20260924-manual.db`: applied `000067_artifact_versions.up.sql`, `000112_career_profile.up.sql`, and `000113_artifact_version_revocation.up.sql`; confirmed the `revoked` column is `NOT NULL DEFAULT FALSE` and an existing row reads `0`; applied `000113` down and up and confirmed the column disappears/reappears while the row remains and still reads `0`.
- The first insertion attempt used column names that do not exist in the formal table (`sequence`, `storage_key`) and failed without changing the temporary DB. The successful exact command was: `sqlite3 /tmp/weknora-t04-migrate-integrated-20260924-manual.db 'INSERT INTO artifact_versions (tenant_id,id,run_id,session_id,digest,object_key,mime,size) VALUES (1,"v","r","s",printf("%064d",0),"k","text/plain",1); SELECT revoked FROM artifact_versions WHERE id="v";' && sqlite3 /tmp/weknora-t04-migrate-integrated-20260924-manual.db '.read migrations/sqlite/000113_artifact_version_revocation.down.sql' 'PRAGMA table_info(artifact_versions);' && sqlite3 /tmp/weknora-t04-migrate-integrated-20260924-manual.db '.read migrations/sqlite/000113_artifact_version_revocation.up.sql' 'PRAGMA table_info(artifact_versions);' 'SELECT revoked FROM artifact_versions WHERE id="v";'` — PASS; observed default `0`, down removed the column, and up restored it with value `0`.
- PostgreSQL runtime migration was not attempted: the available PostgreSQL container is the shared development service, and no isolated PostgreSQL instance was available. No writes were made to shared databases.
- A `migrate -path migrations/sqlite -database 'sqlite3:///tmp/weknora-t04-migrate-integrated-20260924.db' up` probe was attempted and failed before DB access because the installed migrate binary lacks the `sqlite3` driver (`unknown driver sqlite3`). The repository Go migration harness passed as noted above.
- Issuance binds tenant, authenticated owner, run, session, version and expiry in the signed grant; issuance first resolves owned run and readable ready version. Download rechecks signature/expiry, owned run, current active membership, readable/non-revoked version and run binding. These paths and non-disclosing 404 behavior are covered by same-revision focused handler tests.
- New revoke route resolves the authenticated run owner before touching version state, checks the version is readable under tenant/session and matches run, then performs tenant/id-scoped revocation. Repository update is context-bound and idempotently returns not-found if already revoked. Route mount and exact issued-link revocation tests pass.
- No new cancellation-specific contract was introduced. The request context is passed through repository calls; no cancellation defect was observed in the focused path.

## Acceptance status and gaps

- Tenant/owner/resource/fixed-version binding, download-time authorization and revocation checks: supported by code review and passing focused tests.
- Cross-tenant, expired, tampered signature and revoked version rejection: covered by same-revision test suites (in-process HTTP/SQLite-backed tests), not by a live deployment probe.
- Existing Task artifact download regression: covered by `DownloadWorkbenchArtifactGrantResolvesAndStreams` and exact-byte stream regression tests.
- The explicit acceptance request to download a real artifact through a Web deployment and independently compare SHA-256 remains unverified. Environment evidence from the prior same-revision report found no deployment of this revision with an authorized fixture; the available `127.0.0.1:8080` endpoint required unavailable auth and `8081` refused connections. No supported local model/runtime or disposable tenant credentials were established here. The in-process exact-byte/digest assertion does not substitute for this Web acceptance check.

## Risks / conclusion

No backend API-contract, authentication/authorization, data consistency, SQLite migration, cancellation, or error-disclosure defect was identified in the inspected scope. PostgreSQL runtime migration remains unexercised, and the real Web artifact SHA-256 acceptance evidence remains outstanding. Result: `DONE_WITH_CONCERNS`.
