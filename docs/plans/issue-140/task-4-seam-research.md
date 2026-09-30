# Task 4 implementation seam research

Read-only baseline: codex/issue-140-issue30-base @ b5956a9a0f5fb46cb3b8909edd0814006ebe037e. The report did not read Task 3 uncommitted changes and is not an implementation or verification claim.

## Existing Workbench interfaces

- internal/modules/workbench/service/workbench/admission.go:
  - AdmissionCoordinator.Start(ctx, StartInput) (agentruntime.Run, error)
  - LookupRequest(ctx, requestID) (RequestState, error)
  - TaskBudgetPort.Ensure(ctx, tenant, owner, requestID, upper, deadline) (string, error)
  - TaskBudgetPort.ReleaseUnstarted(ctx, reservation) error
  - AdmissionBindingResolver.Resolve(ctx, tenant, actor, StartInput) (TrustedAdmissionBinding, error)
- Start already owns replay for same tenant/actor/request_id, request-hash conflict detection, pending/dispatching/admitted recovery, budget reservation, AgentRunStore.Admit and publish callbacks. Career must call this seam; it must not create a second task runner or edit Workbench admission internals.
- Owned Task read seams:
  - internal/application/repository/agent_run.go: AgentRunStore.GetOwnedRun(ctx, tenantID, ownerID, runID); GetRunForGrantedReader(ctx, tenantID, readerID, runID).
  - internal/application/repository/workbench_task_facts.go: WorkbenchListStore.ReadTaskFactsForRun(ctx, tenantID, ownerID, runID).
  - internal/handler/session/workbench_read.go: owned/granted readers; granted-reader fallback applies only to authorized read paths. Writes, SSE recovery and source events remain owner-only.
  - internal/handler/session/workbench_list.go: OwnedExecutionLister.ListOwnedExecutions(ctx, tenantID, ownerID, WorkbenchExecutionFilter).
- Existing router routes in internal/router/routes_workbench.go expose execution list, detail, start and request lookup. Identity and tenant are server-derived from authenticated context.

## Career contracts and required Task 3 inputs

- repository.ScopeFromContext derives Scope{TenantID, OwnerID} from authenticated Caller and enforces matching identity/tenant/principal; Career endpoints must not accept owner scope from client JSON.
- Current repository.Store baseline only has Get/Put. Receipt/evidence and Artifact grant operations exist, but there are no search, rules, source observation, application-admission or Task-link models yet.
- packages/contracts/src/career and packages/api-client/src/career already include CareerSearchRequest/Receipt, CareerOpportunity + immutable CareerJobSnapshot fields (digest, observedAt, sourceUrl, content, completeness, failureReason), CareerEvaluation tri-state types, CareerApi.search/listOpportunities/lookupRequest.
- Before Task 4 starts, verify Task 3's reviewed production read seam provides profile ID/revision and confirmed facts with provenance; opportunity snapshot ID/revision/digest/source/observation/completeness; evaluation result/revision and hard-conflict evidence. Task 4 must consume, not reimplement, profile confirmation or evaluation.

## Task 4 owned behavior and shared boundaries

- Search module consumes Task 3 opportunity/evaluation snapshots; one-shot search uses CareerSearchRequest requestId and Workbench admission. Result/coverage view preserves original source URLs and checked-at data.
- Source reconciliation merges only strong job/employer/location/batch matches; uncertain duplicates remain separate. Changed/expired/taken-down sources annotate the current observation while historical application snapshot remains fixed. Last success and stale time survive fetch failures.
- Continuous rules require explicit enable/disable/pause/resume; trigger replay is request-ID idempotent.
- Application admission fixes opportunity snapshot, evaluation and profile revision; uses AdmissionCoordinator.Start and existing TaskBudget; retains linking/pending/unknown after uncertain cross-module Task creation. Same request cannot make another Application/Task; distinct hiring batches remain distinct.
- Quota denial blocks new billable Runs only; existing profile/application reads remain available. Do not present NoopTaskBudget as production quota proof.
- Integration-owner-only files: internal/container/container.go, internal/router/router.go and central registration. Current migration runner scans directory pairs; new schema still requires SQLite/versioned pairs and migration tests. Do not make Workbench internals shared writable.
- Task 5 owns formal application/material/submission records. Task 4 may create only the minimal admission/task-link record it needs; if a shared application table or Career receipt schema must change, serialize with Task 5.
- Task 6 owns preparation/timeline/reminder/privacy.

## Parallel split candidate after Task 3 is verified

Three Task 4 substreams may be separated only after Task 3 read interfaces and internal Task 4 DTOs are frozen:
1. Search and source reconciliation: internal/modules/career/search/** and source/**.
2. Continuous rule and quota/admission policy: rule/** and rule-specific admission policy.
3. Application-to-Workbench Task linking: a dedicated adapter/owned subpackage.
These flows must not share central router/container files or the same migration file/schema writer. Main controller serializes migration registration, container construction and router mounting. If durable application rows or receipt-schema changes overlap Task 5, serialize that portion. This is a candidate, not yet an approved implementation split; re-check against Task 3 reviewed interfaces first.

## Decision

Task 4 remains pending until Task 3's backend checkpoint has been independently validated/reviewed and integrated. The stable Workbench admission/read seams are available now; Career domain input fields and exact persistence contracts are not yet verified because Task 3 is still running.
