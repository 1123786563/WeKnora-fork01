# Task 2 Backend Independent Validation

## Result

**DONE_WITH_CONCERNS** — library-level grant and scope behavior plus SQLite migration checks pass. Acceptance requiring an actual owner-resource adapter and download path is not implemented in this checkpoint and is explicitly an integration dependency. Do not mark #142's download acceptance verified from these results.

## Revision and scope

- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-task2-module/WeKnora-fork01`
- Assigned BASE: `715f4a04adbc5ddf637dd34988e3b4c1edc4a900`
- Source checkpoint: `342dee842b8e825d9a84852fc47c13db1d4befec`
- Validated HEAD: `69f000578d9f75f141dbe0ea75790fc173502d87`
- Brief: `.superpowers/sdd/2026-09-28-issue-140-main-port/task-2-brief.md`
- Acceptance references: `docs/plans/issue-140/issues/issue-141.md`, `issue-142.md`, `issue-145.md` (snapshots read 2026-09-24 Asia/Shanghai).
- Source report reviewed: `docs/plans/issue-140/reviews/task-2-module-schema-artifact.md`.
- No source or test files were modified.

## Commands and evidence

All commands below ran from the worktree above.

| Command | Result |
| --- | --- |
| `git rev-parse HEAD` | `69f000578d9f75f141dbe0ea75790fc173502d87` |
| `git show --stat --oneline --no-renames 342dee842b8e825d9a84852fc47c13db1d4befec` | Source commit contains Career module/repository, paired migrations, manifest entries, and Workbench version-grant authority/tests. |
| `go test ./internal/modules/career/... ./internal/modules/workbench ./internal/database -run 'Test(ScopeFromContextRequiresAuthenticatedMatchingPrincipal|Career|VersionArtifactGrant|MigrationVersionsUniquePerTrack)$' -count=1` | PASS; Career repository scope test and packages passed. This regex did not select Workbench grant cases or migration pair/schema cases; those were run separately below. |
| `go test ./internal/modules/workbench -run 'Test(VersionArtifactGrant|NewVersionArtifactGrant)' -count=1` | PASS. HMAC covers tenant, owner, resource, exact version, digest, and expiry; expiry/tampering/ambiguous digest rejection, max TTL, issuance and repeated authorizer calls are exercised. Authorizer is called at issue and every `Authorize` call. |
| `go test ./internal/database -run 'Test(CareerMigrationPairsMatchAcrossTracks|MigrationVersionsUniquePerTrack|SQLiteMigrationsCreateVersionedSchema)$' -count=1` | PASS (`26.516s`). SQLite migration head applies, version uniqueness and up/down table-set pairing checks pass. IDs in source are PostgreSQL/versioned `203` and SQLite `124`. PostgreSQL migration was not applied against a live PostgreSQL service. |
| `make check-backend-architecture` | FAIL (exit 1). Strict decoder processed 17 manifests; no Career strict-YAML diagnostic appeared. Remaining output contains forbidden imports in codedelivery/workbench, uncovered horizontal legacy files, and route-file-coverage diagnostics. I did not run this command at BASE, so baseline status is not independently established here. |
| `go test ./internal/handler/session -run 'Test(DownloadWorkbenchArtifactGrant|DownloadArtifactVersion)' -count=1` | Interrupted after 3m25s with no package output while Go package compilation stalled; no pass/fail result. Earlier implementation report records passes before the final additive authority wrapper, which is not same-revision evidence and is not counted as current validation. |
| `go test ./internal/router -run TestFrontendStaticDoesNotInterceptResourceGrant -count=1` | Interrupted after 2m34s with no output while package compilation stalled; no result claimed. |

## Acceptance assessment

- **Scope/authentication foundation — partial pass:** `ScopeFromContext` requires tenant ID, user ID, and matching authenticated principal ID; `Scope.Validate` rejects empty/invalid scope. Unit test covers absent and mismatched authentication. Repository queries use the composite tenant/owner/fact key. However no Career HTTP handler or router currently invokes this seam, so request-level unauthenticated/wrong-tenant/wrong-owner/missing-resource behavior is not demonstrated.
- **Grant binding and cryptographic checks — pass at authority seam:** canonical HMAC input contains tenant, owner, resource, version, SHA-256 digest and expiry. Tests mutate each bound identity/version/digest field, check expiry and malformed digest/field rejection, and verify TTL capping.
- **Download-time auth/revocation — pass at authority seam only:** `VersionArtifactGrantAuthority.Authorize` first verifies signature/expiry then requires a configured authorizer and calls it. Unit test proves authorizer is re-invoked and can reject after simulated revocation. There is no Career owner-resource catalog adapter and no HTTP endpoint calling `Authorize` before blob access in this checkpoint; actual per-download authorization, deleted/missing/wrong-owner/wrong-version rejection, non-enumerating HTTP errors, and downloadable digest are **not verified**.
- **Client authority — partial pass:** Career scope derives server context, and grant fields must be passed into the server authority. No route exists to verify client JSON/query values are ignored. The authority API accepts identity fields as arguments; caller correctness is a required integration contract.
- **Legacy Task artifact regression — not verified at this exact revision:** session handler test command did not complete. Existing implementation report's earlier pass predates the final additive source wrapper and is not accepted as same-revision evidence. The router static-resource test has no result yet.
- **Migration pairing and IDs — pass on SQLite runner/tests:** version IDs 203 and 124 are present and unique under repository tests; down SQL drops matching Career tables and append-only triggers. PostgreSQL SQL was inspected structurally but not applied to PostgreSQL. `GormStore` has no database-backed Career CRUD test; integration behavior and PostgreSQL compatibility remain unproven.
- **Cancellation/error handling:** repository methods pass `ctx` into GORM. Authority passes the supplied context into its authorizer and propagates errors. No route exists to validate request cancellation, storage-open ordering, status mapping, or error non-enumeration end to end.

## Risks / remaining gaps

1. The source report itself says the grant implementation is a reusable authority, not an endpoint. #142 requires actual download behavior and a real downloaded digest; those criteria remain open until integration.
2. #141 requires tenant provisioning policy, confirmed profile semantics, idempotency/revision conflict behavior, and assessment view isolation. This checkpoint contains only a repository base and schema foundation; those domain behaviors are not present or validated here.
3. Session handler regression and router test outcomes are unavailable at this revision. Architecture diagnostics were not compared against BASE, and PostgreSQL migrations/GORM were not exercised against PostgreSQL.
