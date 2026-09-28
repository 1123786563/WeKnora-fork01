# T35 — Marketplace Evaluation, Privacy Metrics and Publisher Custody Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox syntax for tracking.

**Goal:** Deliver Release-pinned structured Evaluation, privacy-preserving Marketplace Metrics, and Publisher Custody that stops discovery and new Adoption while preserving existing adopter provenance, verified through the public Marketplace HTTP interface.

**Architecture:** Keep Evaluation immutable and keyed to Public Release, with writes restricted to the platform review authority. Build Metrics as an explicit aggregation projection over Adoption, Upgrade and an explicit not_collected state for unavailable adopter error categories; threshold and bucket the data before it reaches any publisher-facing view. Custody preserves immutable packages and publisher provenance while a shared transactional admission predicate prevents new adoption after unlisting, publisher revocation, or verification loss. Compose these capabilities at the existing public catalog/listing HTTP boundary only after the backend contracts are reviewed.

**Tech Stack:** Go, GORM, PostgreSQL/versioned SQL migrations, SQLite SQL migrations, Gin HTTP routes, existing repository/service/handler interfaces and real-migration router tests.

**Spec:** docs/plans/issue30-sweep/issues/issue-65.md; approved docs/specs/2026-09-20-agent-marketplace-domain-model.md §§2, 9–12, 15; CONTEXT.md Marketplace terms; docs/adr/0011-agent-marketplace-release-adoption-boundary.md; Issue #30 DAG and B6 ledger. Dependencies: #60 and #63 verified/integrated; #64 T34 must pass Task6 R2 independent review and validation and be integrated before any T65 production implementation touching shared Marketplace admission/catalog files.

## Global Constraints

- Evaluation is bound to an immutable Release, a test-set identity/version, an environment class, a timestamp and structured results; it helps review/choice but does not grant permissions or guarantee local mapped behavior.
- Publisher unregistration, loss of verification and Listing withdrawal stop new discovery and Adoption. Custody preserves only the minimum immutable Release, Manifest, license, source and Review facts needed by existing Adoptions, subject to license/retention rules.
- Custody never transfers Publisher identity or rewrites PublisherTenantID, PublishedBy, Release lineage, Adoption source or existing tenant-introduced package bytes.
- Security/legal prohibitions use #64 Security Revocation and override ordinary custody continuity.
- Marketplace Metrics are aggregated across adopters and de-identified. No Tenant/member IDs, actor IDs, Task/Prompt/output, local Mapping, model/KB/Connection/tool parameters, raw diagnostic text or publisher telemetry enter publisher-facing metrics.
- Public catalog and detail keep allowlisted DTOs; no raw entities, adopter records or freeform Evaluation/error payloads cross the HTTP boundary.
- Dual-dialect schema changes use distinct next versions after integration re-audit: currently recorded candidates are versioned 000205 and SQLite 000126; confirm current integrated max versions immediately before allocating migration files. Do not fill historical migration gaps.
- Local commits are authorized. Do not push, merge remotely, publish, deploy, or mutate GitHub Issues.

## Review Focus

- A Listing whose Publisher is revoked, unverified, or withdrawn must disappear from both catalog and direct detail, including when a release is pinned explicitly; tests cover both read paths and new-adoption rejection.
- Evaluation remains attached to its exact Release when a Listing current-release pointer advances; include two Releases under one Listing and verify historical Evaluation does not move.
- A one-adopter or below-threshold metric cohort is suppressed, and repeated filters cannot provide a differencing route; exercise HTTP output with tenant/task markers seeded in source rows.
- Adoption/Upgrade aggregation does not read or serialize DiffJSON, raw ErrorMessage, Metadata, Input, Output, mapping IDs, or Task content; use allowlisted lifecycle fields and assert forbidden markers absent. Error-category metrics remain not_collected until immutable Run attribution exists.
- Existing Adoption retains local introduced bytes, source Publisher identity and Review/License lineage through custody; then prove #64 security revocation still blocks new execution/admission.

## Task DAG and Parallel File Preflight

    graph LR
      T1[Task 1: Release Evaluation evidence store] --> T3[Task 3: privacy aggregate projection]
      T1 --> T4[Task 4: catalog and HTTP integration]
      T2[Task 2: custody transactional admission] --> T4
      T3 --> T4

Tasks 1 and 2 may run in independent worktrees after #64 Task6 R2 is verified and integrated: Task1 owns new Evaluation persistence/store/service files, migration pair and schema tests; Task2 owns existing public Marketplace repository/service, publisher Listing lifecycle and shared tenant-guard transaction seam and their test files. They share no files with Task1. Task3 starts after Task1's exact read contract is reviewed and integrated; it owns new Metrics aggregation repository/service/test files and must not modify existing public catalog interfaces. Task4 is serialized last because it updates shared catalog repository read model, interfaces, handler, router/container composition, and highest-stable HTTP test. Shared SQLite database, test ports, or migration outputs are never reused concurrently.

| Boundary | Owned files | Consumes / Produces | Scheduling ruling |
|---|---|---|---|
| Task 1 Evaluation | New internal/types/agent_evaluation_persistence.go; new internal/types/interfaces/agent_evaluation.go; new internal/application/repository/agent_evaluation.go and agent_evaluation_test.go; new internal/application/service/agent_evaluation.go and agent_evaluation_test.go; versioned and SQLite migration up/down; internal/database/migration_sqlite_versioned_schema_test.go | Consumes immutable PublicAgentReleaseEntity IDs and platform reviewer identity. Produces AgentEvaluationEntity and repository contract CreateEvaluation(ctx context.Context, evaluation *types.AgentEvaluationEntity) (*types.AgentEvaluationEntity, error) plus ListEvaluationsForReleases(ctx context.Context, releaseIDs []string) (map[string][]types.AgentEvaluationEntity, error); reject mutable Listing-keyed input. | Parallel with Task2 after #64 gate. Does not edit public catalog interfaces or shared route tests. |
| Task 2 Custody | internal/application/repository/public_marketplace.go and public_marketplace_test.go; internal/application/service/public_marketplace.go and public_marketplace_test.go; internal/application/repository/agent_marketplace_lifecycle.go and agent_marketplace_lifecycle_test.go; internal/application/repository/agent_security_guard.go and agent_security_test.go; no schema migration; use existing VerifiedPublisher, public Listing, source tenant Listing, Release, and Adoption state | Consumes final #64 guarded transaction seam. Produces a multi-tenant ordered guard for adopter and publisher tenant rows; one authoritative discoverability/admission predicate checks VerifiedPublisher state, public Listing state and source Tenant Listing state in list, detail and the IntroduceRelease write transaction. RevokePublisher and tenant Unlist acquire the same publisher-tenant guard. IntroduceRelease runs release/dependency security checks inside the adopter-tenant guarded transaction after resolving/copying the immutable local release. Preserves existing TenantIntroducedReleaseEntity and immutable public Release. | After #64 R2 integration. Task2 adds no schema and uses existing Publisher, public Listing, source tenant Listing, Release and Adoption state. |
| Task 3 Metrics | New internal/application/repository/marketplace_metrics.go and marketplace_metrics_test.go; new internal/application/service/marketplace_metrics.go and marketplace_metrics_test.go; new internal/types/interfaces/marketplace_metrics.go | Consumes Task1 evaluation projection and existing Adoption/Upgrade repositories. Produces MarketplaceMetricsView containing only fixed-window, suppressed/bucketed Adoption and Upgrade values plus an explicit error-category availability state; no identifier or exact raw count fields. | After Task1 review+integration. May proceed in parallel with Task2 only if interface review confirms no shared files/database fixtures. |
| Task 4 Catalog/HTTP | internal/types/interfaces/public_marketplace.go, internal/handler/public_marketplace.go, internal/router/routes_public_marketplace.go, internal/container/container.go, internal/router/routes_public_marketplace_test.go; update T65 plan/ledger evidence index | Consumes Task1 Evaluation read contract, Task2 visibility/admission predicate, Task3 metrics projection. Extends only explicit catalog/detail DTO fields and composes via service. | Serial after Tasks 1–3 verified and integrated. This is the highest stable end-to-end acceptance task. |

## Recorded implementation rulings

1. **Evaluation authority/data shape:** platform SystemAdmin reviewer records structured results against an existing Public Release; one immutable row per release + test-set ID/version + environment class + evaluator + timestamp, with an allowlisted structured result: overall status pass|fail|inconclusive and checks array of {code,status}, where code is one of manifest_completeness, compatibility, license, security, dependency_integrity, privacy and status is pass|fail|not_run; no freeform result text. No publisher self-submitted result is rendered as platform Evaluation. Rationale: the approved spec requires trust evidence but does not define evaluator workflow; using the existing platform review authority avoids inventing publisher authority. Cost if wrong: a narrower trust workflow may need a later approved change.
2. **Metrics disclosure:** use one fixed rolling 30-day window; count distinct adopter Tenant IDs internally, suppress every measure below 5 distinct tenants, and expose only buckets 5-9, 10-19, 20+; never expose exact counts, filters, drilldowns or per-tenant rows. Adopter-level error-category metrics are explicitly not_collected because no safe run-to-Adoption/Release attribution source exists. Do not infer from evaluation outcomes, run status or raw diagnostics. This fails the approved Spec's error-category metric coverage and blocks #65 completion pending a separately designed immutable execution-attribution/event source. Cost if wrong: safe error feedback is deferred; emitting inferred data risks false attribution or tenant/task disclosure.
3. **Custody state:** existing publisher verified|revoked plus Listing listed|unlisted|deprecated are authoritative custody triggers; do not add a second custody owner/state. Keep immutable public Release package and adopter-side introduced package while existing Adoption references it. Security/legal bans remain a hard #64 override. Cost if wrong: licenses may require expiry-driven deletion; preserve current referenced material until a separately verified license-retention rule requires removal.
4. **Migration numbering:** current coordination audit records next versioned 000205 and SQLite 000126. Re-scan after #64 integration; reserve the next numbers once for Task1 only. PostgreSQL runtime cannot be claimed without TRPC_TEST_POSTGRES_DSN; update stale hard-coded migration-head assertions if affected and report unavailable runtime distinctly.

## Task 1: Release-Pinned Structured Evaluation Persistence

**Source:** #65 AC3 and approved Spec §12; Issue #30 marketplace trust signal requirements.  
**Dependencies:** #64 Task6 R2 verified and integrated; current migration tail rechecked.  
**Role:** backend_implementer.  
**Owned files:** internal/types/agent_evaluation_persistence.go; internal/types/interfaces/agent_evaluation.go; internal/application/repository/agent_evaluation.go and agent_evaluation_test.go; internal/application/service/agent_evaluation.go and agent_evaluation_test.go; migrations/versioned/000205_agent_release_evaluations.up.sql and down.sql; migrations/sqlite/000126_agent_release_evaluations.up.sql and down.sql; internal/database/migration_sqlite_versioned_schema_test.go.  
**Consumes:** types.PublicAgentReleaseEntity; existing SystemAdmin reviewer identity from handler context; PostgreSQL/versioned 000205 and SQLite 000126 candidates.  
**Produces:** immutable Evaluation persistence; repository methods CreateEvaluation(ctx context.Context, evaluation *types.AgentEvaluationEntity) (*types.AgentEvaluationEntity, error) and ListEvaluationsForReleases(ctx context.Context, releaseIDs []string) (map[string][]types.AgentEvaluationEntity, error); service methods RecordEvaluation(ctx context.Context, reviewerID string, evaluation types.AgentEvaluationEntity) (interfaces.AgentEvaluationView, error) and ListEvaluationsForRelease(ctx context.Context, releaseID string) ([]interfaces.AgentEvaluationView, error).

- [ ] Add failing repository tests for exact Release binding, required test-set/version/environment/timestamp, duplicate identity conflict, structured result allowlist validation for the six named check codes/status values, and no mutation of existing evaluation after create.
- [ ] Run those tests to confirm the missing Evaluation schema/store behavior fails.
- [ ] Add SQLite and versioned migrations for agent_release_evaluations with release FK/identity, immutable result JSON, evaluator identity, and lookup index; do not persist Task/adopter/local mapping fields.
- [ ] Add AgentEvaluationEntity and repository create/list methods; reject malformed/unknown result fields and duplicate (release_id,test_set_id,test_set_version,environment_class,evaluator_id) identity without overwriting an existing row.
- [ ] Add service validation and SystemAdmin-only write seam following existing public release review conventions; evidence reads return only Evaluation fields needed for the catalog.
- [ ] Extend real SQLite migration/schema tests to assert table columns/indexes and verify migration down/up; test exact Release pin after a second Release is created.
- [ ] Run go test ./internal/application/repository -run 'TestAgentEvaluation' -count=1, relevant service tests, go test ./internal/database -run 'TestSQLiteMigrationsCreateVersionedSchema|TestSQLiteMigrationsUpgradeV4PreservesData' -count=1, git diff --check, and go build ./....
- [ ] Commit only Task1 owned files and write the SDD task report.

**Acceptance mapping:** #65 structured Evaluation is Release-pinned and reviewable at a stable service contract; malformed, duplicate or mutable results are rejected; schema supports both DB dialects.

**Failure handling:** If current reviewer identity cannot be represented without widening an existing security surface, keep the Evaluation writer behind the existing SystemAdmin handler context and request a narrow service constructor change in this Task; do not introduce publisher write authority. If database migration tests cannot load real SQLite migration files, fix the fixture path to the repository's established migrator rather than replacing it with AutoMigrate.

## Task 2: Publisher Custody and Transactional Admission

**Source:** #65 AC2/AC3 and approved Spec §§9–10, 15 scenarios 4 and 9.  
**Dependencies:** #64 Task6 R2 review+validation PASS and integrated exact commit; consumes its final guarded admission/revocation implementation.  
**Role:** backend_implementer.  
**Owned files:** internal/application/repository/public_marketplace.go and public_marketplace_test.go; internal/application/service/public_marketplace.go and public_marketplace_test.go; internal/application/repository/agent_marketplace_lifecycle.go and agent_marketplace_lifecycle_test.go; internal/application/repository/agent_security_guard.go and agent_security_test.go.  
**Consumes:** verified-publisher state, listing lifecycle state, final #64 tenant admission guard APIs, immutable release and TenantIntroducedReleaseEntity rows.  
**Produces:** repository-level shared discoverability predicate and transactional introduction gate; service catalog/detail filters consistent with that predicate; publisher revocation, source Listing unlist, and Introduction serialize on ordered tenant-row guards.

- [ ] Add failing repository/service tests for publisher revocation and source Listing unlist: list/detail hide the record; explicit-release new adoption fails at repository transaction boundary; existing introduced row and local bytes remain unchanged; publisher identity and source lineage remain unchanged.
- [ ] Add deterministic two-connection tests racing publisher revocation and source Listing unlist with IntroduceRelease. Whichever side acquires the publisher tenant guard first completes first; an Introduction acquiring the guard after revocation/unlist is denied. Use repository callback/channel barriers, not sleeps.
- [ ] Run focused tests and record RED against current behavior (revoked Publisher and source-unlisted Listing are currently still represented by the independent public copy, and service checks before the write transaction).
- [ ] Implement one repository visibility/admission predicate over Publisher verified state and Listing listed state. Apply it to catalog query, detail service/read check and the same transaction which creates Introduction/Adoption; retain safe status recheck after any service-layer preflight.
- [ ] Preserve public Release, Review, license, lineage and tenant-introduced bytes; do not rewrite PublisherTenantID/PublishedBy or delete referenced packages. Keep #64 Security Revocation as the stricter deny path.
- [ ] Run go test ./internal/application/repository -run 'TestPublicMarketplace.*Custody|TestPublicMarketplace.*Introduce|TestPublicMarketplace.*Revok' -count=10, relevant service package tests, git diff --check, and go build ./....
- [ ] Commit only Task2's eight owned files and write the SDD task report.

**Acceptance mapping:** #65 custody does not transfer identity or enable new Adoption; existing Adoptions preserve required package and source/history; listing/revocation visibility applies consistently to catalog, detail and write path.

**Failure handling:** If #64 guard does not serialize Publisher/Listing lifecycle mutations with Introduction, stop and report the exact missing transactional seam; do not make a service-only check appear atomic. A schema addition requires a follow-up plan amendment and unique migration reservation before implementation.

## Task 3: Privacy-Preserving Aggregated Metrics Projection

**Source:** #65 AC1; approved Spec §12; CONTEXT.md Marketplace Metrics definition.  
**Dependencies:** Task1 verified/integrated Evaluation outcome contract. Can overlap Task2 only after exact file/interface review proves no overlap.  
**Role:** backend_implementer.  
**Owned files:** new internal/application/repository/marketplace_metrics.go and test, new internal/application/service/marketplace_metrics.go and test, new internal/types/interfaces/marketplace_metrics.go. No edits to existing public catalog interfaces or HTTP/router files.  
**Consumes:** Task1 Evaluation projection and existing Adoption/Upgrade lifecycle repositories. It does not query Task, Run, knowledge-processing, tool-attempt or freeform diagnostic records.  
**Produces:** interfaces.MarketplaceMetricsView with AdoptionBucket, UpgradeBucket and ErrorCategoryAvailability (enum values collected|not_collected); service method MetricsForRelease(ctx context.Context, releaseID string) (interfaces.MarketplaceMetricsView, error); repository method AggregateForReleases(ctx context.Context, releaseIDs []string, since time.Time) (map[string]repository.MarketplaceMetricsAggregate, error), where raw counts are internal-only.

- [ ] Add RED tests with multiple adopter tenants and marker values in Tenant/member IDs, Task title, DiffJSON, ErrorMessage, Metadata, Input, Output, and mapping IDs; assert only aggregate buckets and not_collected availability are returned and all markers/exact counts are absent.
- [ ] Pin boundary tests at 0, 4, 5, 9, 10, 19 and 20 distinct tenants and at the 30-day window boundary; duplicate events from one tenant count once per measure.
- [ ] Implement repository aggregation for Adoption and Upgrade only, selecting only approved source columns and grouping by immutable Release, distinct tenant, measure and fixed 30-day window; never load raw task/evaluation diagnostics. Return error-category availability as not_collected.
- [ ] Implement service transformation to suppressed for cohorts below 5 and the 5-9, 10-19, 20+ buckets; do not return raw counts or caller-controlled dimensions/filters.
- [ ] Verify no response type embeds persistence entities or JSON maps from raw source rows; test serialization has a closed explicit field set.
- [ ] Run targeted repo/service privacy tests with -count=5, go test ./internal/application/repository ./internal/application/service -count=1, git diff --check, and go build ./....
- [ ] Commit only Task3 new owned files and write the SDD task report.

**Acceptance mapping:** #65 metrics are aggregated and de-identified and cannot expose Tenant/member identity or Task content; low-cohort and differencing filters are unavailable by construction.

**Failure handling:** If no existing structured marketplace error source exists, return no error-category metric instead of querying raw Task/Run/knowledge diagnostic records; record this conservative narrowing in the task report. If aggregation query needs a new public filter dimension, reject it unless fixed privacy policy is preserved.

## Task 4: Catalog Trust, Metrics and Custody HTTP Integration

**Source:** #65 all AC; approved Spec §§9–12 and 15.  
**Dependencies:** Tasks 1–3 verified and integrated; #64 Task6 verified/integrated.  
**Role:** backend_implementer.  
**Owned files:** internal/application/repository/public_marketplace.go and public_marketplace_test.go; internal/types/interfaces/public_marketplace.go; internal/handler/public_marketplace.go; internal/router/routes_public_marketplace.go; internal/container/container.go; internal/router/routes_public_marketplace_test.go.  
**Consumes:** Evaluation list service, Metrics projection service, Custody visibility predicate. Add POST /marketplace/public/evaluations, SystemAdmin only, for platform reviewer submissions; catalog and detail expose only the immutable structured result summary.  
**Produces:** catalog/detail DTO fields for reviewer decision/time, compatibility floor and required capability labels, license ID, immutable Evaluation summary and privacy-bucket metrics. Do not add raw metrics query parameters or adopter drill-down route.

- [ ] Extend existing TestPublicMarketplaceCrossTenantEndToEndAndPrivacy or a focused sibling real-router test using existing actual-migration SQLite fixture; seed a verified Publisher, two immutable Releases, reviewer records, structured Evaluation, multiple Adopter tenants and upgrades; error-category availability is explicitly not_collected.
- [ ] Assert strict JSON DisallowUnknownFields DTO shape, exact Evaluation Release linkage after Listing pointer advances, exact public release review decision/time/reviewer, manifest compatibility floor and required capability labels, license ID, immutable Evaluation summary, and correct threshold buckets/suppression.
- [ ] Seed unique marker strings in Tenant IDs/names, Task titles, Prompt/output, DiffJSON, mapping references and diagnostic payloads; assert none occur in catalog, detail or publisher Metrics response. Assert error-category availability is not_collected rather than fabricated zero counts.
- [ ] Exercise publisher revoke, verification loss and unlist: catalog and direct detail both hide; explicit new Adoption returns not-found/denied and writes neither Introduction nor Adoption; existing adopter still reads its local immutable package and provenance.
- [ ] Exercise #64 release/dependency security revocation as an override: custody retention does not reopen admission or permitted execution.
- [ ] Verify route permissions remain Viewer+ for catalog/detail, Admin+ for adoption and SystemAdmin for governance/Evaluation authoring; ensure no endpoint returns a raw entity. Add no security badge unless the public Release revocation projection has a reviewed privacy-safe source; existing #64 local admission/run tests remain the security enforcement evidence.
- [ ] Run go test ./internal/router -run '^TestPublicMarketplace.*(Evaluation|Metrics|Custody|Privacy)$' -count=3, all go test ./internal/router -run '^TestPublicMarketplace' -count=1, go test ./..., go build ./..., and git diff --check.
- [ ] Commit only Task4 owned files and write the SDD task report; update B6 ledger only after all four task reviews and validations pass.

**Acceptance mapping:** #65 AC1 privacy, AC2 custody identity/admission boundary, AC3 real highest-stable HTTP behavior with actual SQLite migrations and production route/service composition.

**Failure handling:** If router fixture cannot represent Evaluation or revocation state without mocks, extend its real migrated fixture; do not substitute unit/static/mock evidence. Missing PostgreSQL credentials must be reported as unrun, while SQLite integration and build can still provide local evidence.

## Acceptance Traceability

| Issue #65 acceptance | Owning task | Required evidence |
|---|---|---|
| Metrics cannot infer Tenant or Task content | Task3 implementation; Task4 HTTP contract | Distinct tenant threshold/bucket tests for Adoption/Upgrade, explicit not_collected error category, marker-based leakage assertions, strict DTO serialization and no arbitrary filter dimensions |
| Custody does not transfer Publisher identity or open new Adoption | Task2 implementation; Task4 HTTP contract | Concurrent repository introduction/revocation schedule, immutable provenance/local-byte assertions, catalog+detail suppression and denied new adoption |
| Highest stable interface proves end-to-end behavior | Task4 | Real migrated SQLite + Gin router + production repository/service/handler composition, no mocks for accepted flow |
| Evaluation belongs to immutable Release | Task1 persistence; Task4 wire test | Two releases under one listing; evaluation ID remains attached to exact source Release after pointer move |

## Pre-Dispatch Self-Review and Shared-File Ruling

- **Spec coverage:** §12 Evaluation is covered by Task1; de-identified Adoption/Upgrade Metrics and privacy boundary are covered by Task3; adopter-level error-category Metrics are NOT covered because current Runs have no immutable Adoption/Release attribution and raw Task/Run data is prohibited as a source. This known gap blocks #65 and #30 completion pending a separately designed provenance/event source. §§9–10 custody triggers/history are covered by Task2; highest stable interface by Task4. The security/legal exception is asserted by Task4 against #64.
- **DAG integrity:** unique Task IDs T1–T4; edges T1→T3, T1→T4, T2→T4, T3→T4; no cycles. Task2 remains externally gated by #64 R2 integration. T1/T2 can be independently implemented in separate worktrees; T3 waits for T1 interface; T4 serializes the shared catalog/HTTP seam.
- **Shared-file scan:** T1 new type/store/service/migration/schema-test files; T2 existing repository/service/tenant-guard files; T3 new aggregation-only files; T4 existing DTO/handler/router/container and one HTTP test. T1 and T2 have no shared owned file; T3 waits on T1; T4 serializes the public wire seam. Migration stream is reserved only for T1; Task2 adds no schema. Router fixture/test state is used only by serial Task4.
- **Exact types/interfaces:** Task1 repository signatures are stated and its service contract lives in internal/types/interfaces/agent_evaluation.go; Task3 returns an explicit bucket-only view and no raw count from internal/types/interfaces/marketplace_metrics.go; Task2 uses sorted lockTenantSecurityRowsTx and rechecks publisher/source/listing/release inside the guarded transaction; Task4 consumes these stable contracts.
- **Review focus:** Review Focus cases for custody, Release pinning and adoption/upgrade Metrics map to Task1–4 tests. Error-category source feasibility is explicitly unresolved: read-only research confirmed there is no existing immutable run-to-Adoption/Release attribution, and raw Task/Run data cannot be used. Record not_collected; this is a known blocker against approved Spec §12, not a completed requirement. No test resource (SQLite DB, HTTP port, generated artifact) is shared across parallel worktrees.
- **Risk ruling:** public Metric minimum cohort 5, fixed rolling window 30 days and buckets are conservative implementation policy because the approved Spec requires de-identification but sets no numeric values. Wrongly permissive values risk re-identification; overly strict suppression reduces publisher utility. This implementation deliberately favors privacy.
