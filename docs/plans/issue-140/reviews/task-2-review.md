# Task 2 independent spec and quality review

Reviewed `715f4a04adbc5ddf637dd34988e3b4c1edc4a900..69f000578d9f75f141dbe0ea75790fc173502d87` (source checkpoint `342dee842b8e825d9a84852fc47c13db1d4befec`) read-only against the Task 2 brief, approved Career spec, ADRs 0015–0019, `CONTEXT.md`, issues #141/#142, implementation report, and independent backend validation. No OCR run was made.

## Findings

### High — Career scope can follow execution tenant rather than authenticated tenant

- **Evidence:** `internal/modules/career/repository/scope.go:11-18` derives `Scope.TenantID` from `TenantIDFromContext` and checks only that the principal ID equals `UserIDFromContext`. `internal/types/context_helpers.go:23-25` explicitly says this is the *execution* tenant and authorization should use `CallerFromContext`. `internal/types/caller.go:38-43` shows `WithExecutionTenant` changes that value while preserving the authenticated caller tenant. The tests cover absent/mismatched principal, but not this supported tenant switch.
- **Impact:** If a Career service call receives a context switched to another execution tenant, the same authenticated user ID authorizes reads or writes in that other tenant. This violates #141 owner/tenant isolation and the approved spec's server-side Tenant permission invariant.
- **Smallest correction:** derive and verify the tenant/owner from the authenticated `Caller` (and required user principal type), reject an execution tenant that differs from the caller's authorized Career space, and add a regression test with `WithExecutionTenant(ctx, foreignTenant)`.

### High — #142 grant download lifecycle and acceptance remain absent

- **Evidence:** `internal/modules/workbench/artifact_signing.go:104-144` adds an interface and HMAC authority, but repository search finds no production implementation of `AuthorizeVersionGrant` and no caller of `VersionArtifactGrantAuthority.Issue` or `.Authorize`. `internal/modules/career/handler/handler.go` is a placeholder. The implementation report's controller hooks explicitly require future issuance/download routes and a catalog adapter. The validator reports no Web download digest and no same-revision Task route regression result.
- **Impact:** No user can obtain or redeem this grant; deleted, revoked, wrong-owner, wrong-version, and digest mismatch conditions have no catalog-backed live decision; the required non-enumerating HTTP response and real Web download digest are unverified. The unit-test fake merely toggles a `revoked` boolean. This checkpoint does not satisfy #142, even though its reusable signing primitive is sound in isolation.
- **Smallest correction:** add the resource/version catalog authorizer and authenticated issuance plus grant-download routes, call `Authorize` on every download before blob open, bind the digest to the served bytes/version, preserve the existing Task URL, and run public API and real Web download digest tests.

### Medium — Invalid TTL silently becomes the maximum lifetime

- **Evidence:** `internal/modules/workbench/artifact_signing.go:64-69` maps `ttl <= 0` to `MaxArtifactGrantTTL` rather than rejecting it; only oversized TTL is tested (`artifact_version_grant_test.go:66-75`).
- **Impact:** A caller attempting zero or negative validity receives a usable 15-minute grant. This can extend authority unexpectedly, especially around cancellation or policy configuration errors. Also, subsecond positive TTL is truncated by Unix-second expiry and may be immediately unusable.
- **Smallest correction:** reject nonpositive TTL, cap only positive over-limit values, and test zero/negative and subsecond behavior (either reject subsecond or round expiry consistently).

### Medium — #141 profile confirmation and provisioning behavior is not implemented

- **Evidence:** `internal/modules/career/repository/repository.go:82-95` only inserts opaque `Payload` under a caller-supplied fact ID; no confirmation/source/revision logic or idempotency receipt use exists. `EnsureSpace` inserts a Career row directly. The only owner test uses `MemoryStore`, while `career_profile_facts.confirmed_at` remains unused in the repository. No Career route is registered.
- **Impact:** The checkpoint cannot establish that proposals are excluded from assessment, repeated confirmations are idempotent, conflicts return the current revision, or first entry follows Tenant provisioning limits, all required by #141. `Put` will report a key conflict on a repeated fact ID rather than the specified receipt/conflict contract.
- **Smallest correction:** treat this as schema foundation only; implement the Tenant provisioning check and profile command/receipt/revision service with database-backed tests and the public Career route before verifying #141.

### Medium — Database-backed CRUD and PostgreSQL migration behavior lack direct validation

- **Evidence:** `repository_test.go` exercises `MemoryStore` only; the GORM path writes `[]byte` to `career_profile_facts.payload`, which is `JSONB` in PostgreSQL and `TEXT` in SQLite (`repository.go:45-95`; both migrations). The migration test matches table names in up/down scripts and applies SQLite, but no PostgreSQL migration or GORM insert/read is exercised. The independent validator confirms this limit.
- **Impact:** The claimed durable repository seam and PostgreSQL deployment compatibility are unproved; driver JSONB parameter handling, foreign keys, and down behavior could fail at runtime. This is a verification gap, not a demonstrated PostgreSQL failure.
- **Smallest correction:** execute paired up/down against PostgreSQL and SQLite and run the real `GormStore` owner-isolation/insert/read tests on the migrated schemas; use an explicit JSON serializer if the PostgreSQL driver rejects raw bytes.

## Positive evidence and limits

- The HMAC canonical form includes tenant, owner, resource, version, digest, and expiry; delimiter/newline and malformed SHA-256 digest fields are rejected. A missing authorizer fails closed, and the authority calls it at issue time and at each `Authorize` call (`artifact_signing.go:44-143`).
- Existing Task grant functions remain in the file without semantic changes. The implementation report records earlier Task handler tests, but the validator's same-HEAD rerun stalled; regression is therefore not established for this checkpoint.
- PostgreSQL and SQLite migrations have unique IDs (`203`/`124`) and matching table sets in up/down scripts. SQLite head and append-only triggers passed the validator's targeted tests. Live PostgreSQL execution was not reported.
- The architecture check exits nonzero on broader repository diagnostics; the validator did not compare those diagnostics to BASE, so no new Career architecture regression is asserted here.

## Verdict

- **Spec compliance: FAIL / incomplete.** Task 2 delivers a useful module/schema and signing foundation, but #142's actual issuance/download lifecycle and #141's profile/provisioning behavior are still open. The scope derivation defect must be corrected before Career data routes use this repository.
- **Code quality: CHANGES REQUIRED.** The tenant authorization defect and TTL semantics need correction; durable CRUD/PostgreSQL behavior and legacy route regression need direct evidence. Do not mark Task 2 verified from the library tests alone.
