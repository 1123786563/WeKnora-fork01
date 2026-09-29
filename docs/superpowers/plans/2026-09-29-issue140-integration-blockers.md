# Issue 140 Integration Blockers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement these tasks. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the merged #140 + issue30-sweep checkout buildable and internally consistent while preserving issue30 as the migration baseline and keeping architecture ownership checks meaningful.

**Architecture:** Keep the exact issue30-sweep commit `db234c5eb171f2dde7427d382b55b503a038f879` as an ancestor. Restore missing Workbench dependency-injection providers, move cross-module consumers behind module-owned interfaces and composition adapters, preserve issue30 migration identifiers while assigning #140-only migration files unique later versions, and reconcile manifests against actual merged code.

**Tech Stack:** Go, GORM, golang-migrate, SQLite/PostgreSQL migration tracks, YAML architecture manifests, TypeScript/Node mini-program tests.

**Spec:** `docs/plans/2026-09-29-issue-140-sweep-baseline-integration.md`; approved #140 DAG `docs/plans/issue-140/2026-09-24-issue-140-dag.md`; `docs/specs/2026-09-23-weknora-job-search-design.md`; ADRs 0015–0018; `CONTEXT.md`; issue30 committed result `docs/plans/issue30-sweep/FINAL-REPORT.md`.

## Global Constraints

- Preserve `db234c5eb171f2dde7427d382b55b503a038f879` as an ancestor and leave the original issue30-sweep worktree and its five untracked files untouched.
- Preserve tenant, owner, membership, run, artifact-version, revocation, legal-hold and audit checks; adapters may translate types but may not weaken behavior.
- Keep module consumers on module-root public contracts; do not suppress Architecture Guard diagnostics or classify new production files as platform by default.
- Preserve issue30 SQLite migrations `000112`–`000123` and versioned/PostgreSQL migrations `000191`–`000202`; preserve special SQLite migration `000114_public_agent_marketplace` because the runner names it explicitly.
- #140 Career/Workbench migration files have not been applied to a database that must be retained (confirmed by user 2026-09-29); renumber only those #140 migration pairs in their existing dependency order.
- Local commits are authorized; do not push, deploy, publish, or mutate GitHub.

## Review Focus

- A missing Workbench provider must fail the container graph test instead of silently leaving an optional route handler nil; exercise grants, research, legacy-list and compliance handlers.
- Research delegation source authorization must reject an out-of-tenant knowledge base before persisting a delegation; retain owner/grant and annotation version checks.
- Compliance audit must remain fail-closed, and the same compliance service must gate session deletion under legal hold/retention rules.
- Cross-module errors and data contracts must preserve `errors.Is`, status mapping and owner-scoped run identity across adapters.
- Migration sequence must load uniquely on SQLite and PostgreSQL, and `up → down → up` must preserve the issue30 marketplace migration's special no-transaction handling.

---

## Execution Baseline

- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-sweep-integration/WeKnora-fork01`, branch `codex/issue-140-sweep-integration`.
- Starting task baseline after Task 2 integration: `e05ce86b8` (descends from checkpoint `8c6578419`, which has exact parents #140 `9bc0d9368` and issue30 `db234c5e`).
- Task 2 implementation commits are integrated: `5bb3eed46`, `e05ce86b8`; focused Artifact handler behavior verification and independent review pass. Full four-package acceptance remains blocked pending this plan's tasks.
- Required issue30 baseline check on detached `db234c5e`: `go test -count=1 ./internal/handler/session ./internal/database` passed. `go test ./internal/container` also passed at this baseline. Architecture Guard already reported 72 diagnostics at `db234c5e`; integrated Task 2 head reports 65. This plan resolves observed blockers rather than attributing them all to the merge.

## Task 3: Restore Workbench Container Providers and Interaction Ports

**Dependency:** Task 2 commits `5bb3eed46` and `e05ce86b8` are integrated; no dependency on Tasks 4–6.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Files:**
- Modify: `internal/container/container.go`
- Modify: `internal/container/workbench.go`
- Test: `internal/container/task_grant_wiring_test.go`
- Test: `internal/container/task_compliance_wiring_test.go`
- Add: `internal/container/workbench_provider_graph_test.go`

**Interfaces:**
- `NewResearchSourceAuthorizer(db *gorm.DB) session.ResearchSourceAuthorizer`
- `NewWorkbenchResearchHandler(db *gorm.DB, runs *repository.AgentRunStore, messages interfaces.MessageService, sessions interfaces.SessionRepository, members interfaces.TenantMemberRepository) *session.WorkbenchResearchHandler`
- `NewWorkbenchLegacyListHandler(db *gorm.DB) *session.WorkbenchLegacyListHandler`
- `NewWorkbenchTaskGrantsHandler(grants *repository.TaskGrantStore, sessions interfaces.SessionRepository, members interfaces.TenantMemberRepository) *session.WorkbenchTaskGrantsHandler`
- `NewTaskComplianceStore(db *gorm.DB) *repository.TaskComplianceStore`
- `NewTaskComplianceService(store *repository.TaskComplianceStore, audit interfaces.AuditLogService) *service.TaskComplianceService`
- `NewWorkbenchTaskComplianceHandler(compliance *service.TaskComplianceService) *session.WorkbenchTaskComplianceHandler`
- `wireTaskDeletionGuard(handler *session.Handler, compliance *service.TaskComplianceService)`
- `NewWorkbenchInteractionService(store *workbenchservice.GormInteractionStore, gate *approval.Gate, streams interfaces.StreamManager, runs *repository.AgentRunStore, admission *workbenchservice.AdmissionCoordinator) *workbenchservice.Service`; use `NewGormCancelPort(runs)`, `NewGormRunRestartPort(storeDB(store), admission)`, and `NewInteractionServiceWithRestart`.

- [ ] Add a real dig graph test requiring grants, research, legacy-list and compliance handlers (not optional fields); add a focused assertion that deletion guard installation delegates to the same compliance service.
- [ ] Run the graph tests and confirm the missing provider/type errors reproduce before implementation; retain current compile errors from unrelated missing DI symbols separately.
- [ ] Implement providers by adapting existing repository, service and handler constructors. Authorize a research source with `GetKnowledgeBaseByIDAndTenant`; normalize lookup failure to out-of-scope. Build the task-grant service inline for research and grants handlers. Keep the task-grant repository explicitly provided.
- [ ] Register all providers in `BuildContainer`. Keep compliance audit required and install the deletion guard through `container.Invoke`.
- [ ] Update the interaction provider to use the durable `AgentRunStore` cancel port and the existing admission coordinator restart port; remove stale `storeDB` use from cancel wiring.
- [ ] Run `go test -count=1 ./internal/container -run 'Test(TaskGrantsAndResearchHandlersBuildable|TaskComplianceWiringRegistered|WorkbenchProviderGraph)$'` and `git diff --check`. Expected: focused tests pass. Run the full package after the other DI providers in this task resolve the package build.
- [ ] Commit `fix(container): restore workbench provider graph` and report exact BASE/HEAD and outputs.

**Acceptance:** Required handler graph resolves with non-nil handlers; research source authorizer is tenant-scoped; compliance audit and deletion guard remain connected; interaction restart/cancel ports use the exact service contracts; focused container tests pass.

## Task 4: Remove Cross-Module Internal Imports

**Dependency:** Starts from `e05ce86b8`; independent file ownership from Tasks 3, 5 and 6.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Files:**
- Modify: `internal/modules/codedelivery/code_platform.go`
- Modify: `internal/modules/codedelivery/dispatcher.go`
- Modify: `internal/modules/codedelivery/gitlab_client.go`
- Modify: `internal/modules/codedelivery/service.go`
- Modify: `internal/modules/codedelivery/gitlab_wire_test.go`
- Modify: `internal/modules/codedelivery/service_dispatch_test.go`
- Modify: `internal/modules/codedelivery/service_gitlab_test.go`
- Modify: `internal/modules/codedelivery/service_prepare_test.go`
- Modify: `internal/modules/workbench/service/workbench/command_queue_next.go`
- Modify: `internal/modules/workbench/service/workbench/command_queue_next_test.go`
- Modify: `internal/container/code_delivery.go`
- Add: `internal/modules/codedelivery/contracts.go`
- Add: `internal/modules/codedelivery/contracts_test.go`

**Interfaces:**
- `codedelivery` owns the narrow provider, credential, action snapshot/outcome and run-session identity contracts it consumes; `internal/container/code_delivery.go` owns adapters from appconnector persistence/service types to those contracts.
- The service must preserve the observable behavior of `errors.Is` checks for dispatch-not-started/unknown outcomes and run not-found/conflict outcomes.
- Workbench command queue owns its command/run outcome mapping or consumes an agentruntime module-root public error contract; it must not import `agentruntime/agent/runtime`.

- [ ] Add contract tests proving adapter mapping preserves action/connection identity, dispatch result state, unknown vs not-started errors, and owner-scoped run session identity; add `errors.Is` tests for queue not-found/conflict mapping.
- [ ] Run `go test -count=1 ./internal/modules/codedelivery/... ./internal/modules/workbench/...` and record the current Architecture Guard import findings before code changes.
- [ ] Move only the values/interfaces required by codedelivery behind its own contracts; implement concrete appconnector adapters in the composition root. Do not expose appconnector repository or service implementations through a new façade.
- [ ] Replace agentruntime internal error imports with a stable module-root error contract or a workbench-owned mapping while retaining `errors.Is` behavior and HTTP/API result mapping.
- [ ] Run `go test -count=1 ./internal/modules/codedelivery/... ./internal/modules/workbench/...` and `git diff --check`. Expected: package tests pass and these source paths no longer produce `forbidden-import` diagnostics.
- [ ] Commit `refactor(modules): keep delivery dependencies behind public contracts` and report exact BASE/HEAD and outputs.

**Acceptance:** Target module packages pass; `codedelivery` and `workbench` production files have no cross-module internal imports; adapter contracts preserve all dispatch/run semantics and error identity.

## Task 5: Assign Unique Versions to #140 Migration Pairs

**Dependency:** User confirmed on 2026-09-29 that #140 Career/Workbench migrations have not been applied to a database that must be retained; preserve issue30's `db234c5e` migration baseline.

**Role:** `mechanical_worker`; validator `backend_validator`; reviewer `reviewer`.

**Files:**
- Rename and update all 19 #140 SQLite migration up/down pairs under `internal/database/migrations/sqlite/`: keep relative dependency order, assign `000124`–`000142`, and retain the migration names.
- Rename and update all 19 #140 versioned/PostgreSQL migration up/down pairs under `internal/database/migrations/versioned/`: same relative order, assign `000203`–`000221`.
- Modify: `internal/database/career_migration_test.go`
- Modify: `internal/database/workbench_migration_test.go`
- Modify: `internal/database/semantic_migration_test.go`
- Test: `internal/database/migration_version_uniqueness_test.go`

**Interfaces:**
- Keep issue30-owned SQLite `000112`–`000123`, versioned `000191`–`000202`, and the exact SQLite `000114_public_agent_marketplace` filename/version unchanged.
- The new #140 SQLite track ends at `000142`; the new versioned/PostgreSQL track ends at `000221`.

- [ ] Enumerate migration up/down filenames from checkpoint `e05ce86b8` and source branch diffs; map the 19 #140 pairs in their existing dependency order before renaming.
- [ ] Add a uniqueness/order assertion proving all filenames load once, issue30 files are unchanged, and #140 files occupy SQLite 124–142 and versioned 203–221.
- [ ] Rename only the identified #140 migration pairs and update every direct filename/version reference found by `rg`.
- [ ] Keep `migration.go` special handling for `000114_public_agent_marketplace` unchanged.
- [ ] Run `go test -count=1 ./internal/database` and `go test -count=1 ./internal/database -run 'TestSemantic(MigrationSQLite|Postgres)UpDownUp'`. Expected: migration source initializes without duplicate versions, SQLite migrations run and rollback/reapply, and PostgreSQL replay passes when `TRPC_TEST_POSTGRES_DSN` is configured (otherwise the integration test reports the missing DSN explicitly).
- [ ] Run `git diff --check`; self-review `git diff --summary` for complete rename pairs; commit `fix(database): sequence issue 140 migrations after sweep baseline`.

**Acceptance:** Both migration tracks have unique versions; issue30 numbering is byte-for-byte unchanged; #140 migrations run after the issue30 baseline in established order; SQLite marketplace migration keeps its special handling; DB tests pass.

## Task 6: Reconcile Architecture Manifests and Route Totals

**Dependency:** Starts from `e05ce86b8`; independent source ownership from Tasks 3–5. Full guard acceptance is checked after Task 4 and Task 6 are integrated.

**Role:** `implementer`; validator `backend_validator`; reviewer `reviewer`.

**Files:**
- Modify as required by exact guard diagnostics: `docs/architecture/moves/agentcatalog.yaml`, `agentruntime.yaml`, `airesource.yaml`, `appconnector.yaml`, `career.yaml`, `channels.yaml`, `commercial.yaml`, `conversation.yaml`, `craft.yaml`, `datasource.yaml`, `execution.yaml`, `identity.yaml`, `insights.yaml`, `knowledge.yaml`, `policy.yaml`, `system.yaml`, `workbench.yaml`.
- Modify: `tools/architectureguard/discovery_test.go`
- Modify: `tools/architectureguard/check_test.go`

**Interfaces:**
- Preserve all 17 currently loaded manifests and their strict schema; encode actual route ownership in `routes`, remaining old horizontal files in `legacy_files` or `move_packages`, and only real lifecycle hooks in `lifecycle_hooks`.
- Derive route, API-key route and hook counts from the merged source graph; do not hard-code stale branch totals or weaken `TestGuardCleanAtHead` diagnostics.

- [ ] Capture exact diagnostics with `go test -count=1 ./tools/architectureguard` and map each route/legacy finding to its owning manifest before editing.
- [ ] Add failing focused assertions for each newly discovered route file and legacy production file; keep the reverse-coverage and strict-schema checks enabled.
- [ ] Update the listed manifests with evidence-backed ownership entries; do not hide production files in `platform` and do not remove findings from the guard.
- [ ] Recompute discovery expectations from the complete merged route/hook graph and update only the literal expected totals and baseline explanation.
- [ ] Run `go test -count=1 ./tools/architectureguard` and `git diff --check`. Expected: `TestGuardCleanAtHead` has zero diagnostics; discovered literal/API-key route totals and hook total match direct discovery output.
- [ ] Commit `docs(architecture): reconcile merged route ownership manifests` and report exact BASE/HEAD and outputs.

**Acceptance:** All actual module route files and legacy files are explicitly owned; full Architecture Guard package passes with zero diagnostics and correct discovered counts; no guard rule is disabled or weakened.

---

## Shared DAG and Integration

| Task | Source Issue | Depends on | Owned scope | Status at plan creation |
|---|---|---|---|---|
| T3 | #140 integration | Task 2 commits integrated | `internal/container/container.go`, `workbench.go`, provider graph tests | ready |
| T4 | #140 integration | Task 2 commits integrated | codedelivery/workbench imports, composition adapter and contract tests | ready |
| T5 | #140 integration | issue30 baseline `db234c5e`; user confirms not deployed | 19 migration pairs + references | ready |
| T6 | #140 integration | Task 2 route counts; guard diagnostics | 15 manifests + guard discovery tests | ready |

All four tasks use distinct writable files and isolated worktrees. Run tasks concurrently; integration is serial and begins only after each task's scoped validator/reviewer passes. Final verification must run on the integrated HEAD: full `go test -count=1 ./internal/handler/session ./internal/container ./internal/database ./tools/architectureguard`, Task 1's complete mini-program suite under supported Node `v22.22.3`, web/mobile regression suites from the verification map, `git diff --check`, and complete-range OCR from `db234c5eb171f2dde7427d382b55b503a038f879` through final HEAD plus workspace content.
