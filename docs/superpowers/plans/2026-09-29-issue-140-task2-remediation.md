# Issue 140 Task 2 Security and Artifact Lifecycle Remediation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct Task 2's Career tenant authorization boundary and complete the #142 exact-version artifact-grant lifecycle before marking the backend foundation verified.

**Architecture:** Authenticated `Caller` is the authorization authority; execution tenant is a repository scope and must match the Career caller's authorized tenant. Workbench owns short-lived grant signing and live authorization, while Career owns the resource/version binding and blob adapter. Signed downloads recheck catalog authorization before opening bytes and preserve the existing Task grant endpoint.

**Tech Stack:** Go, GORM, SQLite/PostgreSQL migrations, Gin, existing Workbench Artifact/session handler interfaces.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`; ADRs 0015–0019; Task 2 in `docs/superpowers/plans/2026-09-28-issue-140-main-port.md`; #141/#142 snapshots; Task 2 reports and review at `docs/plans/issue-140/reviews/task-2-module-schema-artifact.md`, `task-2-backend-validation.md`, and `.superpowers/sdd/2026-09-28-issue-140-main-port/task-2-review.md`.

## Global Constraints

- Authorization derives from authenticated `Caller`, never from client-supplied owner or an execution-tenant override.
- A resource grant binds tenant, owner, resource, exact immutable version, digest, and expiry; every download rechecks live authorization/revocation before bytes are read.
- Unknown issuance never returns a fabricated usable link; deny responses do not reveal resource existence.
- Existing Workbench Task artifact endpoints and URLs remain compatible.
- Do not implement #141 profile confirmation behavior in this remediation; Task 3 owns its proposal/confirmation/revision flow.

## Review Focus

- `WithExecutionTenant` cannot make an authenticated user act in a foreign tenant: regression test with preserved Caller and switched execution scope.
- Zero/negative/subsecond TTL cannot silently grant unexpected access: boundary tests for each value.
- Grant issuance and redemption use a production catalog adapter: integration tests for owner/resource/version/digest and live revocation checks.
- Blob reads occur only after successful HMAC, expiry, and catalog authorization: handler tests assert zero reads for every denied case.
- A successful download returns bytes whose SHA-256 equals the grant/version digest, while the legacy Task download still works.

---

### Task 2R1: Fix Career caller scope and grant TTL boundaries

**Files:**
- Modify: `internal/career/repository/scope.go` and scope tests.
- Modify: `internal/workbench/artifact_signing.go` and grant tests.
- Add SQLite GORM-backed Career repository tests and migration application/down tests in owned test paths.
- No module/container/router production integration in this task.

**Interfaces:** `types.CallerFromContext`, `types.WithExecutionTenant`, existing `Scope`, and `VersionArtifactGrant`.

- [ ] Add a failing test with authenticated Caller tenant 12/user `owner-a`, then `WithExecutionTenant(..., 99)`; `ScopeFromContext` must reject rather than return tenant 99.
- [ ] Add accepted caller tests for matching tenant/user and reject missing caller, mismatched user, empty identity, and foreign execution tenant.
- [ ] Add failing TTL tests for 0, negative, positive subsecond, normal positive, and over-maximum. Reject nonpositive and subsecond durations; cap only positive values above `MaxArtifactGrantTTL`.
- [ ] Add a GORM SQLite test against the applied Career migration: insert/read Career facts/receipts/evidence under one owner, verify another tenant/owner cannot read them, and verify composite FK/append-only constraints.
- [ ] Apply SQLite up/down in a temporary migrated DB; run migration pairing/ID uniqueness tests. If a local PostgreSQL service exists, apply PostgreSQL up/down and run the same repository checks; otherwise record the exact missing service and add static migration parse/shape assertions without claiming live PostgreSQL verification.
- [ ] Run `go test ./internal/career/... ./internal/workbench ./internal/database`; focused auth and migration tests; `git diff --check`; commit Task 2R1.
- [ ] When live PostgreSQL is unavailable, assert the PostgreSQL migration text includes JSONB payload columns, both composite tenant/owner foreign keys, and the append-only update/delete trigger; the SQLite-only test is not enough to claim static shape validation.
- [ ] Bind the PostgreSQL shape fallback per table: receipt response JSONB and scoped FK; profile payload JSONB and scoped FK; evidence payload JSONB, scoped FK, and append-only trigger. A correct global count is insufficient if declarations can migrate between tables.
- [ ] Store and verify version-grant expiry at nanosecond precision so non-millisecond-aligned issue time retains the exact requested TTL; test fractional nanoseconds, one-second and exact maximum TTL boundaries.

### Task 2R2: Complete #142 issuance and versioned download lifecycle

**Dependency:** Task 2R1 reviewed and integrated.

**Files:**
- Modify: `internal/career/**` for production owner/resource/version catalog authorization and issuance service.
- Modify: `internal/workbench/**` only for stable grant contracts if required.
- Modify: `internal/handler/session/artifact_download.go` or a narrowly scoped Career artifact handler for grant redemption before blob access.
- Modify: `internal/container/container.go`, `internal/container/workbench.go`, `internal/router/router.go`, route/handler tests for central assembly owned by this subtask only. Do not alter unrelated central routes.
- Add focused public handler/integration tests; preserve the Task artifact route unchanged.

**Interfaces:** `VersionArtifactGrantAuthority.Issue/Authorize`; Career owner resource/version catalog and byte-source; authenticated Caller; current immutable Artifact storage reader.

- [ ] Add failing handler tests proving no blob read on bad/missing signature, expired grant, cross-tenant, wrong owner/resource/version/digest, revoked/deleted version, or missing authenticated issuer; responses are non-enumerating.
- [ ] Define a production Career artifact catalog adapter backed by owner/resource/exact-version metadata and revocation state; test it against migrated DB/storage fixtures rather than only a boolean fake.
- [ ] Add authenticated grant issuance that derives tenant/owner from Caller, resolves immutable resource/version/digest server-side, caps short TTL, and returns no link on unknown/failed issuance.
- [ ] Add public signed grant download route that verifies HMAC and expiry, calls catalog authorization on every request, then opens and streams the exact immutable bytes; do not accept client authority fields.
- [ ] Add integration test from issuance to HTTP download, compare the downloaded bytes' SHA-256 with the exact stored version digest, revoke/delete, then prove the same URL is denied without reading bytes.
- [ ] Run unchanged Workbench Task grant/download regression and route/resource static-file routing tests after final edits.
- [ ] Run `go test ./internal/career/... ./internal/workbench ./internal/handler/session ./internal/router ./internal/container ./internal/database` plus architecture checks; compare architecture diagnostics against BASE and report any new finding.
- [ ] Provide Web-consumable issuance/download API evidence and a real downloadable version digest when a material fixture exists. If Task 5 material data is not yet available, use a real test artifact served through the registered HTTP route for transport proof and keep Task 5 material digest as a downstream release gate.
- [ ] Commit only owned remediation files and evidence report; request independent backend validation and code review. Task 2 stays running until #142 lifecycle acceptance and Task 2 review both pass.

## Self-review

- #142's acceptance maps to issuance, authorization, route, digest, revocation, non-enumerating denial, and legacy regression checks.
- #141 proposal/confirmation, repeated request and conflict behavior stays assigned to Task 3; Task 2 only supplies secure caller scope, persistence and schema foundations.
- Task 2R2 owns router/container integration explicitly in an isolated worktree because it is the sole serialized owner of this shared wiring.
