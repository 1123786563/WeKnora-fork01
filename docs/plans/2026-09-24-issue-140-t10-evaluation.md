# T10 Evidence-based Career Evaluation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Evaluate a fixed JD against a fixed confirmed Career profile version, showing three-valued hard qualification first and evidence-backed skill, project and intent matches while retaining old evaluations.

**Architecture:** Career Office owns an immutable Evaluation and scoped idempotent receipt. It pins `(opportunityId,snapshotId)` and a captured confirmed-fact version manifest, evaluates only a narrow cited graduation condition from JD text, and treats unsupported conditions as unknown. Web shows hard status and its two-sided evidence before any soft match; no numeric hiring score or model tool call is introduced.

**Tech Stack:** Go/GORM with SQLite and PostgreSQL migrations, Gin, TypeScript contracts/API client, React/TanStack Router.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md` §§2–4, 6.1, 8; `docs/plans/issue-140/issues/issue-150.md`; `docs/adr/0015-job-search-as-weknora-specialist-agent.md`, `docs/adr/0017-immutable-job-and-application-evidence.md`; `docs/plans/issue-140/task-10-architecture.md`, `task-10-research.md`; DAG `docs/plans/issue-140/2026-09-24-issue-140-dag.md`. T08/#146 is verified and integrated; T09 source listing is independent and not a prerequisite.

## Global Constraints

- Only confirmed Career facts may influence qualification. Persist the fact keys, values, revisions, confirmation/source metadata actually used; omit unrelated identity data. `BuildModelInput("qualification_evaluation")` is not an evidence record.
- Freeze the exact Opportunity snapshot and fact versions in every evaluation. A later profile edit or parser release creates a new evaluation; it never rewrites old results.
- `ineligible` takes precedence over unknown and positive soft evidence; unknown is never rendered as eligible. No score is a hiring probability, and the T10 minimum emits no aggregate numeric score.
- A JD is untrusted data. No Agent tool, Task, grant, URL fetch, model prompt instruction, or external side effect is triggered by evaluation.
- Local task commits authorized; no push, merge, deploy, Issue comment/closure. Default serial implementation; each task needs independent review and appropriate validator before downstream work.
- SQLite migration next `000116`, PostgreSQL next `000195` after T08. Update exact architectureguard route totals only after reviewed backend integration; do not edit T08 migration files.

## Review Focus

- Explicit `仅限2027届` plus confirmed graduation 2026 must be `ineligible` with exact JD span and fact revision; confirmed 2027 may be `eligible` for this rule, but unrelated unparsed hard conditions keep overall unknown.
- Missing, unconfirmed, contradictory or malformed graduation year yields `unknown`, and the evidence points to the missing/ambiguous side rather than fabricating a fact.
- An exact request replay after a profile edit returns the original evaluation/receipt; a fresh request captures the new version and old evaluation remains readable.
- A strong soft skill/project/intent match cannot suppress a hard `ineligible` warning or become a probability number. Raw JD remains escaped text.
- Cross-Tenant/user reads and replay are fenced, cancellation/commit ambiguity is recoverable by request ID, and unsupported JD parsing yields `needs_review` rather than a false eligible result.

---

### Task 1: Career Office evaluation and HTTP contract

**Depends:** verified T08 backend. **Owner/validator:** backend_implementer / backend_validator. **Files:** create `internal/modules/career/evaluation.go`, `evaluation_test.go`; modify `internal/modules/career/office.go`, `handler.go`, `internal/router/routes_career.go`, `internal/database/career_migration_test.go`; create `migrations/sqlite/000116_career_evaluations.{up,down}.sql`, `migrations/versioned/000195_career_evaluations.{up,down}.sql`. **Consumes:** `Office.OpportunityEvidence(ctx,opportunityID,snapshotID)`, scoped confirmed `factVersion` rows/profile revision. **Produces:** `Office.EvaluateOpportunity(ctx, EvaluateInput) (EvaluationReceipt,error)`, `Office.Evaluation(ctx,evaluationID) (Evaluation,error)`, `Office.FindEvaluationReceipt(ctx,requestID) (EvaluationReceipt,error)` and additive authenticated routes:

```text
POST /api/v1/career/evaluations
GET  /api/v1/career/evaluations/receipt?requestId=...
GET  /api/v1/career/evaluations/:evaluationId
```

Request `{requestId,opportunityId,snapshotId}` captures current profile revision; optional `profileRevision` may pin a historical revision only if server can reconstruct it. Receipt `{kind:"evaluation_created",requestId,evaluationId,opportunityId,snapshotId,profileRevision,status}`; read includes immutable hard overall/rules, soft evidence, the fixed refs, captured fact evidence, ruleset version/time, and no numeric probability. Replay fingerprint uses the caller's original canonical intent, including whether `profileRevision` was omitted, so a later edit does not change replay identity.

- [ ] RED: Write Office/public HTTP tests for explicit 2027-only JD with confirmed 2026 (`ineligible`), confirmed 2027 (rule eligible), absent year (`unknown`), ambiguous batch (`unknown`), confirmed skill/project/intent literal overlap with evidence spans, unsupported conditions (`unknown`), and high soft match alongside hard failure. Assert each item names snapshot and profile revision/fact revision.
- [ ] RED: Add replay after profile edit (same request returns same ID/output; new request gets new revision), changed intent 409, old read after new evaluation, cross-owner/Tenant denial, cancellation/ambiguous commit receipt lookup, and SQLite migration up/down/up. Run focused tests and record expected failures.
- [ ] GREEN: Implement narrow deterministic cited parser for an unambiguous `仅限 <year> 届` phrase only, a confirmed graduation-year normalizer for `education.graduation_year`/`graduation_year`, and soft exact-evidence matching for confirmed skill/project/preference facts. A JD consisting solely of that recognized condition and whitespace/punctuation can be `eligible` when the confirmed year matches; extra unparsed requirement text makes the overall result `unknown` unless an explicit conflict makes it `ineligible`. Unsupported or conflicting patterns remain unknown. Aggregate hard status with `ineligible > unknown > eligible`, and keep soft results separate.
- [ ] GREEN: In a transaction, scope and pin Opportunity snapshot/profile revision, reconstruct confirmed fact versions at the pinned revision, persist immutable evaluation output and receipt atomically. Scope all reads; reconcile uncertain writes with bounded same-ID receipt lookup. Add migrations/AutoMigrate and handler error mapping; no network or tool side effect.
- [ ] VERIFY: `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...`, SQLite up/down/up, `git diff --check`; report SQL/Postgres runtime availability. Independently review Spec and quality before integration.

### Task 2: Architectureguard route baseline

**Depends:** reviewed Task 1 integrated. **Owner/validator:** mechanical_worker / backend_validator. **Files:** `tools/architectureguard/discovery_test.go`, dated `docs/architecture/moves/README.md`, focused guard tests only as necessary. **Consumes:** exact new Career route discovery. **Produces:** current route totals with drift detection and historical 633/644/647 preserved.

- [ ] PRECHECK: Measure runtime and focused guard mismatch after Task 1; record actual delta and ownership violations. Do not assume only three routes if the integrated code differs.
- [ ] RED/GREEN: Update only the exact current count for audited registrations, keep historical checkpoints and Career file ownership unchanged.
- [ ] VERIFY: focused guard, runtime guard, `go test -count=1 ./...`, `git diff --check`; independent review/validation before integration.

### Task 3: Typed Web evaluation API

**Depends:** reviewed Task 1 wire and Task 2 integration. **Owner/validator:** frontend_implementer / frontend_validator. **Files:** `packages/career-core/src/contracts.ts`, `contracts.test.ts`, `packages/api-client/src/career.ts`, `career.test.ts`. **Consumes:** actual Go receipt/read JSON and routes. **Produces:** strict `EvaluationReceipt`/`Evaluation` decoders and `client.career.evaluateOpportunity`, `evaluationReceipt`, `evaluation` methods, independent of profile mutation `CareerAction`.

- [ ] RED: Add contract fixtures for all hard statuses, fixed refs, fact revisions and evidence spans; reject malformed/missing provenance, unknown enum, probability-like aggregate field, and wrong receipt discriminator. Add method/path/body/encoding tests.
- [ ] GREEN: Implement additive strict decoders and client methods matching reviewed backend JSON. Do not reinterpret unknown as eligible; preserve immutable raw evidence as inert strings.
- [ ] VERIFY: focused contract/client tests, `pnpm typecheck:web`, `git diff --check`; independent review/validation before integration.

### Task 4: Evaluation action/detail in Career conversation

**Depends:** reviewed Task 3 integrated. **Owner/validator:** frontend_implementer / frontend_validator. **Files:** `apps/web/src/career/` evaluation UI/tests, narrow `OpportunityPage.tsx` or chat result hook, `apps/web/src/router.tsx`, `routes.tsx`/tests, local CSS. **Consumes:** typed fixed Opportunity evidence and evaluation client. **Produces:** explicit Evaluate action on a fixed JD, result/detail route by evaluation ID, old/new evaluation views.

- [ ] RED: Test 2026 versus 2027-only warning first, missing graduation unknown, evidence links to exact JD and confirmed fact version, soft skill/project/intent evidence after hard status, no probability wording, reload old result after profile edit and new evaluation, scope/logout/403 fencing, malformed response. Include narrow-width DOM/CSS visual check.
- [ ] GREEN: Add explicit evaluation action and stable result link. Render hard status prominently before soft evidence; do not let a soft match hide ineligible. Show provenance, unknown reasons and `needs_review` without calling tools. Keep old evaluation route readable by fixed ID.
- [ ] VERIFY: focused Web tests, `pnpm typecheck:web`, `pnpm test:web`, `pnpm build:web`, `git diff --check`; independent review/validation, then controller live API/SQLite browser acceptance with synthetic confirmed profile and JD.

## Shared-file and task preflight

Task 1 alone owns Career Go, migrations and route registrations. Task 2 begins only after Task 1 review/integration and owns guard files only. Task 3 alone owns shared TS contract/client after the Go wire is frozen. Task 4 starts after Task 3 review/integration and owns Web UI/router files, with any `OpportunityPage` shared edit reviewed in that task. Tests use separate temporary SQLite paths; no shared database/port/build directory is used by parallel writers. Each downstream task consumes a reviewed commit integrated into the same serial execution branch. The backend Task Brief must restate its exact owned files and source SHA; reviewer and validators never change production or test source.

## Coverage and failure handling

The first four #150 criteria map to Task 1 Office tests and Task 4 browser behavior. A failure to reconstruct pinned facts or validate source spans returns a recoverable error/`needs_review`, never a favorable assessment. If a reviewer finds a valid medium-or-higher issue, create a numbered focused repair plan, use the same specialized implementer in the isolated worktree, independently re-review and revalidate, then integrate. T10 stays running until all four tasks and live browser evidence pass. Parent #140 OCR still follows the complete DAG, not this one task.
