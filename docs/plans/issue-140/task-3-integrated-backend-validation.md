# T03 / #141 Integrated Backend Validation

- Revision: `e22b6e4ae461acc307b446023c935291c416c622`
- Validation date: 2026-09-24 Asia/Shanghai
- Scope: assigned #141 backend checkpoint; no source or test-source edits.
- Issue source: `docs/plans/issue-140/issues/issue-141.md` (GitHub REST snapshot, 2026-09-24).
- Approved behavior source: `docs/specs/2026-09-23-weknora-job-search-design.md`.

## Commands and results

| Command | Result |
| --- | --- |
| `git rev-parse HEAD` | `e22b6e4ae461acc307b446023c935291c416c622` |
| `go test ./internal/modules/career -count=1` | PASS, `ok .../internal/modules/career 2.093s` |
| `go test ./internal/router -count=1` | PASS, `ok .../internal/router 3.297s` |
| `git status --short` | clean before report creation; only this permitted validation report is added |

The Career suite opens isolated in-memory or `t.TempDir()` SQLite databases. `NewOffice` runs GORM `AutoMigrate` in these tests, so the Career schema migration path was exercised on disposable SQLite. No shared development database was used. In this validation run, `TestSlowConcurrentMutationAcrossPostgresConnections` was not run against PostgreSQL because no isolated DSN was supplied. The earlier `task-3-backend-r4-report.md` separately records an isolated `postgres:17` Docker runtime twin and passing PostgreSQL concurrency runs at the earlier backend checkpoint. That evidence is relevant prior evidence, but it is not a PostgreSQL run performed by this integrated validator or at this integrated HEAD.

## Evidence against acceptance

- **Owner-only / cross-tenant scope:** `Handler.scope` derives user and tenant from authenticated request context, calls `TenantMemberService.ListByTenant` on every request, and rejects non-singleton/non-owner/mismatched membership. `Office` additionally requires an existing tenant+owner `career_spaces` row on reads and writes. `TestCareerHTTPRechecksTenantMembershipOnEveryRequest`, `TestCareerTenantRequiresSingleActiveOwner`, `TestOfficeRejectsUnclaimedOrCrossOwnerCareerSpace`, and `TestCareerFactsStayIsolatedAcrossUsersAndTenants` pass.
- **Router authentication binding:** `NewRouter` installs `middleware.Auth(...)` before constructing `/api/v1`; `RegisterCareerRoutes` is called under that authenticated group. The separate `/api/v1` API-key authorizer is installed before route registration. Router package tests compile and pass. This is source-path inspection plus package tests; no full `NewRouter` request with production auth dependencies was exercised.
- **Unconfirmed proposals excluded:** `TestUnconfirmedProposalIsNotConfirmedFact` verifies proposals remain in `Proposals` and absent from `Facts` until confirmation.
- **Confirm/replay and revision conflict:** `TestConfirmProposalResolvesAndRetainsConfirmationHistory` proves identical request replay returns identical receipt without duplicating fact/version, changed-content reuse conflicts, and resolved proposal cannot confirm again. `TestRevisionConflictHasCurrentValueAndIdempotency` proves stale revision returns current revision. `TestConfirmedFactVersionsAreImmutable` proves history remains append-only.
- **Unknown result / receipt path:** `TestPostConflictReceiptLookupDeadlineReturnsUnknownWireError` and `TestPostConflictReceiptLookupCancellationReturnsUnknown` verify cancellation/deadline during replay lookup maps to an unknown outcome with request ID and HTTP 504. `TestHTTPErrorBodiesMatchSharedFixtures` checks receipt-not-found and conflict wire errors. `Receipt` scopes lookup by tenant, user and request ID.
- **Cancellation and database error distinction:** focused Career tests pass for deadline/cancellation unknown outcomes and preservation of unrelated database errors.

## Gaps and risks

- No browser/client run was part of this backend checkpoint; stale response suppression after switching spaces, login-to-reopen consistency, and user-visible recovery remain outside verified evidence. #141 explicitly requires a Web browser flow.
- No PostgreSQL DSN was supplied; database-specific row locking and uniqueness behavior under PostgreSQL are not covered here.
- The actual router auth chain was inspected and the router package passed, but a mounted end-to-end HTTP request through real JWT/API-key middleware was not constructed. Handler fixture tests exercise the authenticated context and membership boundary directly.
- Personal workspace creation policy is enforced upstream by the existing authenticated tenant lifecycle; this Career module claims a tenant only after verifying its current single owner. No end-to-end tenant creation/entitlement flow was exercised.
- Read `.superpowers/sdd/2026-09-24-issue-140-implementation/task-3-backend-rereview-5.md`: it independently marks round-5 Spec compliance and code quality PASS for `bd2dcbd..2e966ac`, specifically resolving the post-conflict cancellation/deadline mapping. It did not run tests and explicitly leaves full Web behavior and router/auth integration separate. This is earlier-scope review evidence; it does not independently review the complete integrated T03+T04+docs HEAD recorded here.
- Read `.superpowers/sdd/2026-09-24-issue-140-implementation/task-3-backend-r4-report.md`: its actual isolated PostgreSQL 17 runtime evidence is distinguished above from this run. Its report also describes earlier focused package/race and contract checks.
- **Mounted HTTP probe feasibility:** the existing suite has no `NewRouter(...)` test (repository search: `rg -n 'NewRouter\\(' --glob '*_test.go' .` returned no matches). Existing route tests typically mount an individual route group with a test-only context injector, bypassing `middleware.Auth`. `NewRouter` wires a large `RouterParams` graph and many route registrations, while `middleware.Auth` itself requires realistic tenant/user/member/API-key service implementations and a valid token or API key. There is no existing narrow harness that can issue both unauthenticated and cross-tenant requests through the real mounted Career route without constructing that dependency graph. Building such a harness would require adding test source or relying on real auth services/credentials; per assignment neither was done. Therefore the mounted unauthenticated/cross-tenant HTTP probe was not feasible within the no-source-edit/no-shared-service constraints. The router middleware ordering and Career registration path were verified by source inspection, and direct authenticated-context handler/membership behavior is covered by existing Career tests.

## Status

**DONE_WITH_CONCERNS** — backend behavior is supported by passing focused Go suites and SQLite migration execution; earlier isolated PostgreSQL 17 concurrency evidence exists at the prior backend checkpoint. This validator did not rerun PostgreSQL or migration behavior at integrated HEAD. Full mounted middleware request, upstream tenant entitlement, and required Web browser acceptance remain unverified.
