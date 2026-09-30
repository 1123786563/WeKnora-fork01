# T14 / #155 codebase research (read only)

Research date: 2026-09-24 (Asia/Shanghai). Source tree: integration worktree at
`/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`, HEAD
`2eba6c141`. Scope: existing Workbench Task creation and recovery seams, Career
Office opportunity/evaluation evidence, migration conventions, and Web navigation.
No production or test files were changed.

## Verified facts

### Requirements and invariants

* `docs/plans/issue-140/issues/issue-155.md` requires an application to pin job
  snapshot, profile revision and evaluation; allow explicit hard-fail continuation
  with a persistent warning and exclude it from qualified metrics; retain a linking
  state when cross-module Task creation is unknown and reconcile by original
  request ID; allow separate applications for separate batches while making retries
  idempotent.
* Approved `docs/specs/2026-09-23-weknora-job-search-design.md` sections 3, 4 and
  8 define the same evidence/version rules. `docs/adr/0016-one-task-per-job-application.md`
  establishes one independent WeKnora Task per application; ADR-0017 requires
  immutable job/application evidence. `CONTEXT.md` defines Task as the durable
  work unit and preserves tenant/user authority on the server.

### Workbench Task creation API and authority

* HTTP write seam is `POST /api/v1/workbench/executions`, registered by
  `internal/router/routes_workbench.go:RegisterWorkbenchStartRoutes`; request
  lookup is `GET /api/v1/workbench/executions/requests/:request_id`.
  The routes use authenticated viewer/API-key policy. The start handler is
  `internal/modules/session` (router field `WorkbenchStartHandler`), while the
  domain admission implementation is `internal/modules/workbench/service/workbench/admission.go`.
* `workbench.StartInput` has seven request fields: `session_id`, `agent_id`,
  `target_id`, `workspace_ref`, `space_id`, `request_id`, `text`, and
  `budget_upper`; `Binding` is server-only. `AdmissionBindingResolver.Resolve`
  is the trusted seam for tenant/actor ownership, target, credentials, parent,
  pricing and funding. Client-supplied binding/authority must not be trusted.
* `AdmissionCoordinator.Start` persists/locks a `workbench_requests` row keyed by
  `(tenant_id, actor_id, request_id)`, and `resumeExisting`/`admitPending` resume
  the same request. `LookupRequest` returns `pending`, `dispatching`, `admitted`,
  or `rejected` plus `run_id`/reason. A repeated request with changed input is
  rejected as an idempotency conflict; same input resumes the original request.
  This is the natural Task creation target for an Application linking operation.

### Existing Career Office evidence and evaluation seams

* `internal/modules/career/opportunity.go:ImportJD` accepts `requestId`, raw JD,
  source label/reference and creates immutable `career_opportunities`, an
  observation, and a snapshot. `OpportunityReceipt` returns
  `opportunityId`, `observationId`, `snapshotId`, status and acquisition time.
  `OpportunityEvidence` reads by tenant/user/opportunity/snapshot and returns raw
  text, SHA-256, extracted fields, source and acquired time. It therefore supplies
  the pinned job snapshot reference required by T14.
* `internal/modules/career/evaluation.go:EvaluateOpportunity` accepts
  `requestId`, `opportunityId`, `snapshotId`, optional `profileRevision`, verifies
  the scoped snapshot and observation, pins the current/profile revision, and
  stores `career_evaluations` with receipt/evaluation evidence. Receipt includes
  `evaluationId`, opportunity/snapshot IDs, profile revision and status
  (`eligible`, `ineligible`, `unknown`). Hard rules are represented separately
  from soft matches, with job evidence and confirmed profile fact evidence.
* Career HTTP routes are in `internal/router/routes_career.go`: POST
  `/career/opportunities/import`, GET `/career/opportunities/receipt`, GET
  `/career/opportunities/:opportunityId` (snapshot is a query parameter in the
  handler), POST `/career/evaluations`, GET evaluation receipt/detail. Career
  handler scope enforces a single owner-only personal tenant via
  `validateOwnerOnlyCareerTenant`; this must remain the authority boundary for
  application records.

### Idempotency and unknown-result recovery

* Career opportunity/evaluation writes use a durable scoped request receipt with
  SHA-256 fingerprint. Same `(tenant,user,requestId)` and same payload returns the
  prior receipt; changed payload returns `ErrIdempotencyConflict` (HTTP 409).
  If commit/ack may have been lost, bounded reconciliation queries the same
  request ID under a context detached from cancellation; failure to determine the
  outcome returns `OutcomeUnknownError` (HTTP 504) with `requestId`.
* The shared client seam `packages/domain/src/mobile/submission.ts` codifies the
  corresponding rule: persist request ID and input digest before network, never
  re-POST an existing pending/unknown request, reconcile with lookup, and only
  explicitly rejected submissions can be retried with a fresh request ID.
  `task-form.ts` preserves the draft until dispatch is confirmed. T14's
  application linking state should follow this exact pattern and retain the
  original request ID through unknown Task creation.

### Migrations

* Versioned PostgreSQL migrations are sequential paired files under
  `migrations/versioned/`; SQLite has a parallel numbered series under
  `migrations/sqlite/`, with matching up/down files. Career currently ends at
  versioned 195 (`000191_career_profile`, `000193_career_source_revisions`,
  `000194_career_opportunities`, `000195_career_evaluations`) and SQLite 116
  for the equivalent feature. `internal/database/career_migration_test.go`
  asserts table/column/index shape and down-migration behavior.
* Existing Career tables use tenant/user scope and unique request receipts. New
  application/linking storage should preserve this convention and add explicit
  uniqueness for the application identity (job/opportunity plus recruiting batch)
  and/or the request receipt, rather than relying on client deduplication.

### Web navigation

* `apps/web/src/routes.tsx` currently recognizes `/platform/career`,
  `/platform/career/opportunities/:opportunityId` (with snapshot ID), and
  `/platform/career/evaluations/:evaluationId`; `apps/web/src/router.tsx` mounts
  `CareerPage`, `OpportunityEvidencePage`, and `EvaluationDetailPage` under the
  same paths. No application route or task deep-link route was found in these
  career route declarations.
* The existing Workbench routes are API routes; application UI should navigate to
  a stable application detail surface and deep-link the associated `run_id` only
  after the server has returned `admitted`. Unknown/linking must not render a
  ready Task link.

## Inferences / integration risks

1. T14 needs a new vertical application seam that composes Career Office evidence
   lookup/evaluation with Workbench admission. There is no existing application
   model/table or Career-to-Workbench linking service in this tree.
2. The application identity must include recruiting batch. `OpportunityFields.Batch`
   is extracted from the pinned snapshot, but extraction can be `unknown`; the
   implementation must define whether an explicit user batch key is required or
   whether unknown batches are represented distinctly. Silently treating unknown
   as one batch would violate the separate-batch acceptance criterion.
3. Workbench `request_id` is the recovery authority for Task creation, while the
   application itself needs its own durable linking status and request ID. A
   transaction cannot safely assume Career application persistence and Workbench
   admission share one DB transaction; design for `linking`/`unknown` and a
   reconciliation endpoint/job that calls Workbench lookup.
4. Hard-condition continuation can use `EvaluationReceipt.Status` plus stored
   hard rules, but a continuation override must be recorded as an explicit user
   fact/decision on the application. It must not mutate the original evaluation or
   make an ineligible evaluation appear eligible.
5. Web navigation currently has no application route. Adding one will require
   route parser, TanStack route tree, and page wiring changes; the existing route
   patterns provide the canonical `/platform/career/...` prefix.

## Recommendations to parent agent

* Reuse `OpportunityEvidence` and `Evaluation` IDs as immutable foreign references
  (or validated scoped references) on the application record; do not copy mutable
  latest JD/profile values.
* Generate one stable application-create `requestId`, persist the application in
  `linking` before calling Workbench Start, pass the same request ID to the Task
  admission request, and reconcile via `GET .../requests/:request_id` after timeout
  or 504. Same batch + same request should return the existing application/Task;
  a different batch must have a distinct application key.
* Add both PostgreSQL and SQLite paired migrations and extend the existing career
  migration shape tests. Preserve tenant/user owner-only checks in every handler.
* Add Web application navigation alongside current career routes and gate Task
  deep-link rendering on an admitted `run_id`; show persistent warning and
  non-qualified state for explicit hard-fail continuation.

## Source pointers

* Requirements: `docs/plans/issue-140/issues/issue-155.md`
* Approved behavior: `docs/specs/2026-09-23-weknora-job-search-design.md`
* Domain/architecture: `CONTEXT.md`, `docs/adr/0015-job-search-as-weknora-specialist-agent.md`,
  `docs/adr/0016-one-task-per-job-application.md`, `docs/adr/0017-immutable-job-and-application-evidence.md`
* Career: `internal/modules/career/opportunity.go`, `internal/modules/career/evaluation.go`,
  `internal/modules/career/handler.go`, `internal/router/routes_career.go`
* Workbench: `internal/modules/workbench/service/workbench/admission.go`,
  `internal/router/routes_workbench.go`, `packages/domain/src/mobile/submission.ts`,
  `packages/domain/src/mobile/task-form.ts`
* Schema/navigation: `migrations/versioned/000191_career_profile.up.sql` through
  `000195_career_evaluations.up.sql`, `migrations/sqlite/000112_career_profile.up.sql`
  through `000116_career_evaluations.up.sql`, `internal/database/career_migration_test.go`,
  `apps/web/src/routes.tsx`, `apps/web/src/router.tsx`
