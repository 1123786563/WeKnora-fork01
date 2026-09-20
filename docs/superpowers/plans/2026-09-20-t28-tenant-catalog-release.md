# T28 Tenant Catalog Immutable Release Publication Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a Tenant Agent Author freeze a concrete Agent Version, submit a sanitized Tenant Catalog Release, obtain an authorized Tenant Admin/Owner review, and publish an immutable Release behind a mutable Listing pointer.

**Architecture:** Add a new Tenant Marketplace aggregate beside the legacy `PublishedExpert` surface. Persist an immutable `AgentVersion` snapshot and append-only Submission/Review/Release records; only a Listing's current Release pointer changes. Canonical sanitized bundle bytes are stored by digest under a Release-specific path. PostgreSQL and SQLite migrations enforce uniqueness; service transactions bind approval to the exact reviewed digest. A small Web surface consumes the HTTP API so the capability is usable end to end.

**Tech Stack:** Go 1.26, Gin, GORM, existing versioned PostgreSQL and SQLite SQL migrations, existing Agent/Expert serialization utilities, TypeScript `@weknora/api-client`, React 19 Web app (`apps/web`) with the existing Node `tsx --test` runner, SQLite migration-backed integration tests.

**Spec:** `docs/specs/2026-09-20-mobile-ai-office-design.md`; normative model: `docs/specs/2026-09-20-agent-marketplace-domain-model.md`; architecture: `docs/adr/0011-agent-marketplace-release-adoption-boundary.md`; Ticket: GitHub issue #58.

## Global Constraints

- Agent Marketplace distributes sanitized immutable Agent Release snapshots, not mutable CustomAgent definitions or running agents.
- A Tenant Adoption can only arise after the explicit Tenant Catalog governance boundary; Adoption/Variant are outside this Ticket.
- Release content excludes knowledge-base bindings, model configuration, credentials, Sandbox, Memory, Task/session content and other Tenant-local resources.
- Release Manifest, Dependency Lock, license, portable content and digest are fixed together; changing any of them creates a new Submission and Release.
- AgentVersion is owned by the Agent domain; the Marketplace stores only its immutable version ID and source digest.
- A repeated publication creates a new immutable Release and advances the Listing pointer; it never replaces an older Release or its files.
- Submission, Review and Release records are append-only; Tenant Listing is the only mutable catalog pointer in this Ticket.
- Tenant role authorization remains server-side. Existing Tenant Admin/Owner roles are the reviewer authority for this first governance surface; Author submission requires existing agent edit permission.
- Database uniqueness and transactions/CAS enforce multi-instance correctness. Process-local keyed mutexes are not correctness boundaries.
- Existing `/market/tenant/experts` and `PublishedExpert` refresh-in-place APIs remain backward-compatible legacy behavior; new routes and tables use separate names and semantics.
- The closed-loop evidence exercises the highest stable HTTP interface with real SQLite migration, repositories, service, authorization and filesystem materialization; mocked repository/service tests supplement but do not replace it.

## Review Focus

- A Tenant ID or Agent ID supplied by a caller must not allow reading another Tenant's version or Listing; pin this in Task 5's route integration test.
- Changing portable content after review must fail digest/CAS validation before publishing; pin this in Task 4's transactional service test.
- Two concurrent submissions for the same source/version must not overwrite one another's Release or snapshot path; pin this in Task 3's repository uniqueness test.
- Sensitive local bindings hidden inside `CustomAgent.Config` must be absent from canonical bundle bytes and Manifest; pin this in Task 2's exporter test using populated forbidden fields.
- Re-publication must preserve Release 1 bytes and identity while creating Release 2 and moving only the Listing pointer; pin this in Task 5's HTTP E2E test.

---

## File Structure

| Path | Responsibility |
|---|---|
| `internal/types/agent_marketplace.go` | Submission, Review, Release and Listing persistence entities/value types. |
| `internal/types/interfaces/agent_marketplace.go` | Application-facing service contracts and views. |
| `internal/types/agent_version.go`, `internal/types/interfaces/agent_version.go` | Agent-domain immutable Tenant Agent Version snapshot and service contract, separate from Marketplace ownership. |
| `internal/agent/experts/agent_release.go` | Allow-listed portable export, canonical JSON encoding, digest calculation and safe materialization. |
| `internal/application/repository/agent_version.go`, `internal/application/service/agent_version.go`, `internal/handler/agent_version.go` | Agent-domain version freeze/read behavior that supplies a fixed source to Marketplace. |
| `internal/application/repository/agent_marketplace.go` | Tenant-scoped GORM repository and transactional pointer/append operations. |
| `internal/application/service/agent_marketplace.go` | Submission, review, release and catalog application behavior. |
| `internal/handler/agent_marketplace.go` | Strict HTTP DTO validation and current-principal extraction. |
| `internal/router/routes_agent_versions.go`, `internal/router/routes_agent_marketplace.go` | Agent version and Marketplace submission/review/catalog routes with RBAC. |
| `internal/router/router.go`, `internal/container/container.go` | Route registration and production dependency wiring. |
| `migrations/versioned/000178_agent_versions.up.sql`, `000178_agent_versions.down.sql` | PostgreSQL immutable AgentVersion table and rollback. |
| `migrations/sqlite/000099_agent_versions.up.sql`, `000099_agent_versions.down.sql` | SQLite twin AgentVersion schema and rollback. |
| `migrations/versioned/000179_tenant_agent_marketplace.up.sql`, `000179_tenant_agent_marketplace.down.sql` | PostgreSQL Submission/Review/Release/Listing tables and rollback. |
| `migrations/sqlite/000100_tenant_agent_marketplace.up.sql`, `000100_tenant_agent_marketplace.down.sql` | SQLite twin Marketplace schema and rollback. |
| `internal/database/migration_sqlite_versioned_schema_test.go` | Keep the existing migration parity matrix covering new tables/indexes. |
| `internal/application/repository/agent_marketplace_test.go` | Real SQLite migration, tenant predicate, uniqueness and transaction tests. |
| `internal/application/service/agent_marketplace_test.go` | Service state transitions, redaction and digest-bound review tests. |
| `internal/router/routes_agent_marketplace_test.go` | Actual HTTP boundary, RBAC and full publication lifecycle integration test. |
| `packages/contracts/src/agents/versions.ts`, `packages/api-client/src/agents/versions.ts` | AgentVersion response parser and typed freeze/read client. |
| `packages/contracts/src/marketplace/tenant-releases.ts` | Wire DTO parser only for Release Submission/Review/Listing responses. |
| `packages/api-client/src/marketplace/tenant-releases.ts` | Typed Tenant Marketplace HTTP client adapter. |
| `packages/api-client/src/marketplace/tenant-releases.test.ts` | HTTP path/body/envelope tests. |
| `apps/web/src/agent-marketplace/agent-marketplace-api.ts` | Web feature API adapter built on shared api-client, no direct fetch duplication. |
| `apps/web/src/agent-marketplace/AgentVersionActions.tsx` | Freeze source version and start release submission from the Agent editor. |
| `apps/web/src/agent-marketplace/TenantReleaseReview.tsx` | Tenant Admin/Owner review and publish panel in the existing Tenant Market tab. |
| `apps/web/src/agents/AgentEditorModal.tsx`, `apps/web/src/market/MarketPage.tsx` | Wire Author action and Reviewer queue into existing React governance surfaces. |
| `apps/web/src/agent-marketplace/*.test.tsx` | Author and Reviewer behavior at the feature component/API boundary, using the existing DOM/jsdom test helpers. |

Migration tails verified at plan creation: PostgreSQL `000177`, SQLite `000098`; this plan reserves PostgreSQL `000178_agent_versions` and `000179_tenant_agent_marketplace`, plus SQLite `000099_agent_versions` and `000100_tenant_agent_marketplace`. Recheck both tails immediately before implementation and stop for coordination if either number has been consumed.

## Interface Contract

The Agent domain freezes the source independently of Marketplace. HTTP handlers derive tenant and user principals from authenticated request context:

```go
type AgentVersionService interface {
    FreezeAgentVersion(ctx context.Context, tenantID uint64, actorID, agentID string) (AgentVersionView, error)
    GetAgentVersion(ctx context.Context, tenantID uint64, versionID string) (AgentVersionSnapshot, error)
}
```

Marketplace then exposes this separate contract:

```go
type AgentMarketplaceService interface {
    SubmitRelease(ctx context.Context, tenantID uint64, actorID, versionID string, input SubmitReleaseInput) (ReleaseSubmissionView, error)
    ListReviewQueue(ctx context.Context, tenantID uint64) ([]ReleaseSubmissionView, error)
    ReviewSubmission(ctx context.Context, tenantID uint64, actorID, submissionID, expectedDigest string, decision ReleaseReviewDecision, reason string) (ReleaseReviewResult, error)
    ListTenantCatalog(ctx context.Context, tenantID uint64) ([]TenantListingView, error)
}

type ReleaseDependencyResolver interface {
    Resolve(ctx context.Context, tenantID uint64, version AgentVersionSnapshot) (DependencyLock, error)
}
```

`AgentVersion` belongs to the Agent domain and snapshots one Tenant-scoped `CustomAgent` configuration and source digest. Marketplace references that ID but does not own or mutate the source Agent Version. `AgentReleaseSubmission` stores fixed `agent_version_id`, canonical Manifest/Dependency Lock/license, bundle digest and review state; `AgentReleaseReview` records reviewer, digest, decision, reason and time; `AgentRelease` stores immutable semantic version/digest/Manifest/Dependency Lock/license/bundle reference; `AgentMarketplaceListing` stores Tenant scope, source Agent ID, display data, state and `current_release_id`. Constraints include unique `(tenant_id, agent_id, version_number)`, unique `(tenant_id, source_agent_id)` Listing, unique `(listing_id, release_number)`, unique `(listing_id, semantic_version)`, unique immutable release digest per listing, idempotent reviewer decision key `(submission_id, reviewer_id, reviewed_digest)` and foreign keys from pointers to Release. Listing publication advances with a transaction that checks the expected prior pointer; append-only rows have no update methods.

Author-supplied Manifest fields are `semantic_version`, `display_name`, `summary`, `supported_languages`, `use_cases`, `non_use_cases`, `capability_requirements`, `data_categories`, `external_side_effects`, `minimum_weknora_capability`, `license_id`, and `change_notes`. Listing ID, Release ID, Tenant Publisher identity, publication time and digest are server-generated. Dependency Lock entries are server-resolved from the immutable AgentVersion's portable Skill/subagent references and contain dependency type, stable identity, concrete version, digest and license. If a dependency has no immutable distributable version, submission fails with the missing dependency named; caller-supplied digests are never trusted.

Agent version freeze/read routes live under `/api/v1/agents/:id/versions`; Marketplace routes live under `/api/v1/marketplace/tenant/release-submissions`, `/api/v1/marketplace/tenant/release-submissions/review-queue`, `/api/v1/marketplace/tenant/release-submissions/:id/review`, and `/api/v1/marketplace/tenant/catalog`. An approved review atomically appends the Review, inserts the immutable Release and advances the Listing pointer; there is no separate publication command that could leave an approved Submission unlisted. `packages/contracts` parses stable wire forms; `packages/api-client` owns request serialization and errors; Web components consume only the api-client feature adapter.

## Implementation Tasks

### Task 1: Freeze an immutable Tenant Agent Version in the Agent domain

**Files:**
- Create: `internal/types/agent_version.go`, `internal/types/interfaces/agent_version.go`, `internal/application/repository/agent_version.go`, `internal/application/repository/agent_version_test.go`, `internal/application/service/agent_version.go`, `internal/application/service/agent_version_test.go`, `internal/handler/agent_version.go`, `internal/router/routes_agent_versions.go`, both SQL migration pairs listed above
- Modify: `internal/database/migration_sqlite_versioned_schema_test.go`
- Test: `internal/application/repository/agent_version_test.go`, `internal/application/service/agent_version_test.go`

**Interfaces:**
- Consumes: `CustomAgent` and tenant-scoped `GetAgentByIDAndTenant` from existing custom-agent service.
- Produces: `FreezeAgentVersion(ctx, tenantID, actorID, agentID)` in the Agent domain, returning immutable ID, version number and source digest for Marketplace to reference.

- [ ] **Step 1: Write a failing SQLite repository test** that applies versioned SQLite migration 000099 and proves two consecutive frozen versions for one Agent have increasing version numbers and immutable snapshots, while another Tenant cannot fetch either version.
- [ ] **Step 2: Run `go test ./internal/application/repository -run TestAgentVersionFreeze -count=1`**; expect failure because AgentVersion entity, migration and repository are absent.
- [ ] **Step 3: Add the append-only version entity and twin migrations**. Persist a JSON snapshot plus canonical source digest and `(tenant_id, agent_id, version_number)` uniqueness. Freeze in a transaction; do not add a `CustomAgent.Version` mutable field or mutate legacy market tables.
- [ ] **Step 4: Implement the AgentVersion application service and route** with tenant-scoped reads, `g.OwnedAgentOrAdmin()` authorization and a version-freeze operation that cannot update/delete prior snapshots.
- [ ] **Step 5: Run `go test ./internal/application/repository ./internal/application/service ./internal/handler ./internal/router -run 'AgentVersion|AgentVersions' -count=1` and `go test ./internal/database -run 'TestSQLiteMigrationsCreateVersionedSchema|TestSQLiteMigrationsUpgradeV4PreservesData' -count=1`**; expect cross-tenant reads to fail closed and dialect parity to pass.
- [ ] **Step 6: Commit** as `feat(agent): persist immutable tenant agent versions`.

### Task 2: Export a canonical sanitized Release bundle

**Files:**
- Create: `internal/agent/experts/agent_release.go`, `internal/agent/experts/agent_release_test.go`
- Modify: `internal/types/agent_marketplace.go` only if named portable value types belong beside persistence entities.
- Test: `internal/agent/experts/agent_release_test.go`

**Interfaces:**
- Consumes: immutable AgentVersion snapshot from Task 1.
- Produces: `BuildAgentReleaseBundle(version AgentVersionSnapshot, input ReleaseMetadata, lock DependencyLock) (AgentReleaseBundle, error)` where bundle includes canonical `Manifest`, `DependencyLock`, portable payload bytes and SHA-256 digest.

- [ ] **Step 1: Write failing exporter tests** using source config populated with KB IDs, model IDs, credential-shaped strings, Sandbox config ID, Memory flags, Task transcript markers, portable prompt/tools/subagents and stable metadata. Assert the portable fields remain and every forbidden marker is absent from all serialized bundle bytes. Also reject missing license, unsupported language, empty use cases, absent minimum capability, and a dependency without a stable version/digest/license.
- [ ] **Step 2: Run `go test ./internal/agent/experts -run TestBuildAgentReleaseBundle -count=1`**; expect missing exporter symbol.
- [ ] **Step 3: Implement allow-list projection** from the fixed AgentVersion snapshot using the Manifest fields above. Validate every provided server-resolved `DependencyLock` entry is immutable, redistributable and licensed. Canonically serialize the exact envelope (portable payload + Manifest + Dependency Lock + license) and calculate SHA-256 over those bytes.
- [ ] **Step 4: Add digest stability and forbidden-data tests**; identical snapshots must produce identical bytes/digest, while any portable behavior, lock, Manifest or license change alters the digest.
- [ ] **Step 5: Run `go test ./internal/agent/experts -run TestBuildAgentReleaseBundle -count=1`** and the package suite; expect all pass.
- [ ] **Step 6: Commit** as `feat(marketplace): canonicalize sanitized release bundles`.

### Task 3: Persist Submissions, Reviews, Releases and Tenant Listings

**Files:**
- Modify: `internal/types/agent_marketplace.go`, `internal/types/interfaces/agent_marketplace.go`, `internal/application/repository/agent_marketplace.go`, `internal/application/repository/agent_marketplace_test.go`, both 000179/000100 migration pairs, `internal/database/migration_sqlite_versioned_schema_test.go`
- Test: `internal/application/repository/agent_marketplace_test.go`

**Interfaces:**
- Consumes: AgentVersion persistence and Release bundle from Tasks 1–2.
- Produces: `CreateSubmission`, `ListReviewQueue`, `ReviewAndPublishTx(expectedPriorReleaseID, submissionID, expectedDigest, decision)` and tenant-scoped `ListTenantCatalog` repository methods.

- [ ] **Step 1: Write failing repository tests** for Submission digest immutability, review append-only behavior, unique per-Listing Release numbers, tenant isolation, publication pointer CAS and preservation of an older Release.
- [ ] **Step 2: Run `go test ./internal/application/repository -run 'TestAgentMarketplace(Submission|Review|Publish|TenantScope)' -count=1`**; expect failures for absent tables and methods.
- [ ] **Step 3: Add twin migrations 000179/000100** for `agent_marketplace_listings`, `agent_release_submissions`, `agent_release_reviews`, and `agent_releases`, with matching foreign keys, unique constraints and indexes. Append `.down.sql` rollback files for each dialect and parity assertions.
- [ ] **Step 4: Implement repository queue and review transaction methods**. `ReviewAndPublishTx` reloads the Submission and compares the reviewed digest. Approval appends the Review, allocates the next Release under database uniqueness/transaction protection, inserts the immutable Release, and conditionally advances `current_release_id`; rejection appends a Review but creates no Release. Do not rely on `keyedMutex`.
- [ ] **Step 5: Run the named repository tests and `go test ./internal/database -run 'TestSQLiteMigrationsCreateVersionedSchema|TestSQLiteMigrationsUpgradeV4PreservesData' -count=1`**; expect both dialect schemas and rollback path to pass.
- [ ] **Step 6: Commit** as `feat(marketplace): persist submissions reviews releases and listings`.

### Task 4: Implement author and reviewer service transitions

**Files:**
- Create: `internal/application/service/agent_marketplace.go`, `internal/application/service/agent_marketplace_test.go`
- Modify: `internal/types/interfaces/agent_marketplace.go`, `internal/application/repository/agent_marketplace.go` only to complete the declared transaction interface.
- Test: `internal/application/service/agent_marketplace_test.go`

**Interfaces:**
- Consumes: AgentVersion service from Task 1, `ReleaseDependencyResolver`, Release bundle builder and Marketplace repository from Tasks 1–3.
- Produces: the five methods in the `AgentMarketplaceService` contract; reviewer must be Tenant Admin/Owner at the route boundary, author must pass existing edit permission.

- [ ] **Step 1: Write failing service tests** for submit from immutable AgentVersion, reviewer-required Manifest/Dependency Lock/license, stale digest rejection, reject-with-reason, approval atomically creating a Release, and duplicate decision retry returning the same result without overwriting bytes.
- [ ] **Step 2: Run `go test ./internal/application/service -run TestAgentMarketplace -count=1`**; expect missing service/API failures.
- [ ] **Step 3: Implement transitions**. Load the previously frozen AgentVersion, resolve portable dependencies from its immutable snapshot, generate the canonical bundle once, persist the bundle by digest, bind review to that digest, and call the repository transaction for reviewer decisions. Approval atomically records Review + Release + Listing pointer. Store the immutable bundle at a Release-specific digest path; never use `PublishedSnapshotDir` or mutate legacy `PublishedExpert` rows.
- [ ] **Step 4: Add compensation for storage/database boundaries**: materialize to a temporary path, verify digest, atomically rename to digest path, then persist the Release transaction; on transaction failure leave no catalog pointer and remove only the unreferenced staged object.
- [ ] **Step 5: Run the service suite** with `go test ./internal/application/service -run TestAgentMarketplace -count=1`; expect state and digest tests to pass.
- [ ] **Step 6: Commit** as `feat(marketplace): implement tenant release governance service`.

### Task 5: Expose the Tenant catalog workflow through authenticated HTTP

**Files:**
- Create: `internal/handler/agent_marketplace.go`, `internal/router/routes_agent_marketplace.go`, `internal/router/routes_agent_marketplace_test.go`
- Modify: `internal/router/router.go`, `internal/container/container.go`
- Test: `internal/router/routes_agent_marketplace_test.go`

**Interfaces:**
- Consumes: service contract from Task 4 and standard Go authenticated request context.
- Produces: authenticated version-freeze, submission, review-queue, review-and-publish and catalog HTTP routes described above. Tenant ID and actor ID always come from trusted context, never request body.

- [ ] **Step 1: Write a failing `httptest` API lifecycle test** using real SQLite migration/repositories/service and a temporary bundle directory. The sequence must freeze version → submit → inspect review queue → review exact digest with approval → list and assert 201/200 responses, immutable Release receipt and updated Listing pointer.
- [ ] **Step 2: Write failing authorization tests** proving a different Tenant returns not-found/forbidden, Viewer cannot submit/review, Contributor can submit only an Agent they may edit, and Tenant Admin/Owner can inspect the review queue and approve/reject.
- [ ] **Step 3: Run `go test ./internal/router -run TestTenantAgentMarketplace -count=1`**; expect unregistered route and handler failures.
- [ ] **Step 4: Add strict request DTO decoding and route registration**. Use existing `g.OwnedAgentOrAdmin()` for version-freeze/submission, `g.Admin()` for review queue/review (approval is publication)/listing management, and `g.Viewer()` for catalog reads. Add full-access API-key policy to reads and the existing manage-agent policy to mutations. Wire repository/service/handler via container without modifying legacy market routes.
- [ ] **Step 5: Run the API lifecycle/authorization test and `go test ./internal/handler ./internal/router ./internal/container`**; expect the full real SQLite-backed request flow and guard matrix to pass.
- [ ] **Step 6: Commit** as `feat(api): expose tenant agent release workflow`.

### Task 6: Add typed AgentVersion and Marketplace clients

**Files:**
- Create: `packages/contracts/src/agents/versions.ts`, `packages/contracts/src/agents/versions.test.ts`, `packages/contracts/src/marketplace/tenant-releases.ts`, `packages/contracts/src/marketplace/tenant-releases.test.ts`, `packages/api-client/src/agents/versions.ts`, `packages/api-client/src/agents/versions.test.ts`, `packages/api-client/src/marketplace/tenant-releases.ts`, `packages/api-client/src/marketplace/tenant-releases.test.ts`
- Modify: `packages/contracts/src/index.ts`, `packages/api-client/src/index.ts`
- Test: all four new parser/client test files.

**Interfaces:**
- Consumes: AgentVersion and Marketplace HTTP DTOs from Tasks 1 and 5.
- Produces: wire-only AgentVersion and Release parsers; `createAgentVersionsApi(request)` with `freezeVersion` and `getVersion`; `createTenantReleaseApi(request)` with `submit`, `listReviewQueue`, `review`, `listCatalog`.

- [ ] **Step 1: Write failing parser/client tests** for required AgentVersion identifiers and digest, immutable Release Submission/Review identifiers, malformed envelopes, exact paths/methods/bodies and the Review response carrying a Release only for approval.
- [ ] **Step 2: Run `pnpm exec tsx --test packages/contracts/src/agents/versions.test.ts packages/contracts/src/marketplace/tenant-releases.test.ts packages/api-client/src/agents/versions.test.ts packages/api-client/src/marketplace/tenant-releases.test.ts`**; expect missing modules/exports.
- [ ] **Step 3: Implement wire-only types and parsers** with no product workflow logic in contracts; implement API client through existing `ClientRequest` with no new HTTP transport.
- [ ] **Step 4: Run all four new tests and `pnpm typecheck:shared`**; expect parser validation and path typing to pass.
- [ ] **Step 5: Commit** as `feat(api-client): add tenant release contracts`.

### Task 7: Provide the Tenant Author and Reviewer Web workflow

**Files:**
- Create: `apps/web/src/agent-marketplace/agent-marketplace-api.ts`, `apps/web/src/agent-marketplace/AgentVersionActions.tsx`, `apps/web/src/agent-marketplace/AgentVersionActions.test.tsx`, `apps/web/src/agent-marketplace/TenantReleaseReview.tsx`, `apps/web/src/agent-marketplace/TenantReleaseReview.test.tsx`.
- Modify: `apps/web/src/agents/AgentEditorModal.tsx` to add the Author action and `apps/web/src/market/MarketPage.tsx` to add a Tenant Release review section. Preserve unrelated edits in the worktree.
- Test: new feature component and API adapter tests using the repository's existing `node --import tsx --test` runner.

**Interfaces:**
- Consumes: `createTenantReleaseApi` from Task 6 and authenticated Web user's Tenant role.
- Produces: Agent Author can freeze/select a version and submit a Release; Tenant Admin/Owner can inspect fixed Manifest/Dependency Lock/license/digest and approve or reject with a reason. Approval immediately returns the created immutable Release.

- [ ] **Step 1: Write failing component/API tests** for version freeze and submit, role-gated review controls, required review decision, digest shown unchanged from review detail through approval, and rejection reason.
- [ ] **Step 2: Run `pnpm --filter @weknora/web test`**; expect missing module/component import errors from the newly added feature test.
- [ ] **Step 3: Implement the Author action and Tenant review panel** in the existing Agent editor and Market Tenant tab. Do not add Adoption/Variant/upgrade flows beyond this Ticket. Use shared API client; the Admin/Owner review action displays the exact reviewed digest and returns the Release produced by the atomic approval transaction.
- [ ] **Step 4: Run `pnpm --filter @weknora/web test` and `pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit`**; expect the feature suite and typecheck to pass.
- [ ] **Step 5: Commit** as `feat(web): add tenant release author and review flow`.

### Task 8: Verify the complete release invariants and integration gates

**Files:**
- Modify: only existing tests or implementation files required by failing integration evidence.
- Test: Go marketplace repository/service/router packages; shared contracts/api-client; Web feature suite.

**Interfaces:**
- Consumes: complete HTTP and Web workflow from Tasks 1–7.
- Produces: verified evidence for all Ticket #58 acceptance criteria, PostgreSQL/SQLite migration parity, redaction boundary, immutable re-publication and old API compatibility.

- [ ] **Step 1: Run complete ticket suites**: `go test ./internal/agent/experts ./internal/application/repository ./internal/application/service ./internal/handler ./internal/router ./internal/database ./internal/container`; `pnpm exec tsx --test packages/contracts/src/agents/versions.test.ts packages/contracts/src/marketplace/tenant-releases.test.ts packages/api-client/src/agents/versions.test.ts packages/api-client/src/marketplace/tenant-releases.test.ts`; `pnpm --filter @weknora/web test`.
- [ ] **Step 2: Run the SQLite-backed HTTP lifecycle twice**; expect Release 1 digest and file bytes to remain identical, Release 2 to receive a new immutable identity, and only `Listing.current_release_id` to advance.
- [ ] **Step 3: Run migration parity/rollback checks**; expect new indexes/foreign keys present on both dialects and down migration to remove only T28-owned schema.
- [ ] **Step 4: Run legacy compatibility tests** with `go test ./internal/application/repository ./internal/application/service ./internal/handler ./internal/router -run 'PublishedExpert|TenantExpertMarket'`; expect existing legacy behavior unchanged.
- [ ] **Step 5: Inspect serialized release bytes** from the integration fixture and assert no forbidden KB/model/credential/Sandbox/Memory/Task marker occurs; preserve digest and response evidence in the test output.
- [ ] **Step 6: Commit** any final test-only changes as `test(marketplace): prove immutable tenant release lifecycle`.

## Self-Review

1. **Spec coverage:** immutable sanitized Agent Release, Manifest/Dependency Lock/license, Tenant review, Listing pointer, repeat publication and cross-tenant isolation are covered in Tasks 1–8; Public Marketplace, Adoption, Variant, mobile catalog, upgrade and Marketplace metrics stay with later Tickets.
2. **Placeholder scan:** No TBD or unresolved implementation task remains. The exact migration suffixes and route prefix are explicit; any pre-implementation migration collision is a coordination stop rather than silent renumbering.
3. **Type consistency:** Task 1 creates AgentVersion persistence; Task 2 consumes its snapshot; Task 3 stores Submission/Review/Release; Task 4 declares and implements service methods; Task 5 maps those methods to HTTP; Task 6 parses those exact HTTP objects; Task 7 consumes the client; Task 8 verifies the same full flow.
4. **Review Focus:** tenant access, digest CAS, concurrent immutability, forbidden-field redaction and re-publication preservation each have an explicitly named test owner.

## Handoff

This plan covers only Ticket #58. After approval, execute with `superpowers:subagent-driven-development`; each Task gets an isolated implementer and a separate task review. Existing `/market/tenant/experts` remains a legacy surface during this Ticket. Tenant Admin/Owner is the concrete reviewer authority because those are the existing governance roles and the Release model assigns Tenant Catalog review to a Tenant-authorized reviewer.
