# T10/#150 Career Evaluation research

Research date: 2026-09-24 (Asia/Shanghai). Scope: read-only codebase and approved design research for #150. No application or remote Issue changes were made.

## Verified requirements and facts

- #150 consumes a confirmed Career profile version and an immutable JD snapshot. It requires three-valued hard qualification (`符合`/`不符合`/`待确认`), evidence pointers to both versions, scores that are never described as hiring probability, and new evaluations after profile changes while retaining old evaluations. A missing graduation date is unknown; a 2026 graduate against a 2027-only JD is ineligible.
- Approved job-search spec `docs/specs/2026-09-23-weknora-job-search-design.md` states that hard constraints precede skill/project/intent matching, explanations must distinguish user-confirmed facts, source facts, and model inference, and model output must be structurally validated with a human review path. ADR-0015 and ADR-0017 establish the Career boundary and immutable evidence semantics.
- Confirmed profile facts are exposed by `Office.Open(ctx)` as `View{Revision, Facts, Proposals}`. `Fact` carries `Key`, `Value`, `Revision`, `Source`, `Confirmation`, and `ConfirmedAt`; only rows in `career_facts` are confirmed. `Office.History(ctx,key)` reads append-only `career_fact_versions` ordered by revision and returns the same public `Fact` shape. Profile scope is enforced by `WithScope`, `requireSpace`, tenant and owner checks.
- The profile revision is advanced transactionally by `Office.mutate`; each confirmation writes the current fact, a `factVersion`, a change event, and an idempotent receipt. Therefore an evaluation can bind to `View.Revision` plus selected fact revisions, and old fact versions are available for later reads.
- T08/#146 provides the fixed JD seam: `Office.OpportunityEvidence(ctx, opportunityID, snapshotID)` returns exact raw text, SHA-256, extracted fields, source, acquisition time, status, and immutable opportunity/observation/snapshot IDs. It scopes reads to the current Career owner and rejects missing or mismatched snapshots with `ErrOpportunityNotFound`. HTTP is `GET /api/v1/career/opportunities/:opportunityId?snapshotId=...`.
- T08 extraction currently defaults all fields to `unknown` and `needs_review` unless an injected extractor returns valid known/unknown fields. This is useful for the evaluator: missing/unknown batch, graduation, or requirements must remain `待确认`, never be guessed from absent data.
- Existing `Office.BuildModelInput(ctx, "qualification_evaluation")` filters confirmed facts to education, experience, skill, and preference and redacts identity/passport numbers. It intentionally strips source/confirmation metadata and is therefore not sufficient by itself for evidence binding; evaluation code should use `Open`/`History` for provenance and only use model input as an optional inference input.
- There is no existing qualification, matching, scoring, or evaluation persistence/service in `internal/modules/career`; repository search found only the model-input purpose name and resume extraction. Existing Web opportunity page (`apps/web/src/career/OpportunityPage.tsx`) displays evidence but has no evaluation UI.

## Likely public seam and ownership

Recommended Career Office seam: a pure or Office-owned operation receiving a scoped immutable snapshot reference and a profile revision, for example `EvaluateOpportunity(ctx, opportunityID, snapshotID, requestID/expectedProfileRevision)`, returning a versioned evaluation object. It should internally load `OpportunityEvidence`, the current or requested confirmed fact version set, and persist an immutable evaluation record keyed by tenant/user and evaluation ID. The returned object should include:

- evaluation ID, created time, profile revision, opportunity ID, snapshot ID, and evaluator/model version;
- hard qualification items with `status: qualified|not_qualified|unknown`, rule/requirement text, selected fact key plus fact revision (or explicit missing evidence), and JD snapshot pointer;
- skill/project/intent matches as evidence-backed explanations with explicit inference status and missing evidence;
- an aggregate preparation/match score only if clearly labelled as a non-probability signal; hard `not_qualified` must remain prominent and cannot be overridden by score.

This seam should be placed beside `opportunity.go` and `office.go` (new `evaluation.go` is the likely module boundary), with HTTP handler/routes added only after the Office contract is stable. Existing handler scope and `writeError` provide tenant/owner fencing and error mapping. New persistence needs SQLite and Postgres migrations plus SQLite schema validation in `office.go`; immutable evaluation rows should retain the exact profile revision/fact revisions and opportunity snapshot IDs.

## Existing tests and useful fixtures

- `internal/modules/career/office_test.go`: confirmation, revision conflicts, replay, history, tenant isolation, and concurrent writes.
- `internal/modules/career/opportunity_test.go`: exact snapshot reads, replay, scope fencing, extraction unknowns, HTTP contract, and concurrent import behavior.
- `internal/modules/career/model_input_test.go` (if extended): model-purpose filtering and redaction behavior.
- `internal/modules/career/handler_test.go`: owner-only tenant checks and per-request membership revalidation.
- `packages/career-core/src/contracts.ts` and `packages/api-client/src/career.ts`: public wire decoder/client patterns for any new evaluation contract; Web E2E should follow `OpportunityPage.test.tsx` evidence and narrow-layout conventions.

## Dependencies and risks

- Direct dependency: T08/#146 must be integrated and its Opportunity evidence contract must remain stable. T03/#141 Career workspace/owner scope is already integrated.
- The requirement says "confirmed profile version" but the current API has only current `Open` plus per-key `History`; no atomic public method returns a complete historical profile snapshot. A new evaluator must either persist a complete fact-version manifest at evaluation creation or add a read seam that atomically captures all confirmed facts and their revisions. Selecting facts later would violate reproducibility.
- T08 extracted fields are usually unknown. Qualification tests should use a deterministic test extractor or fixture, while production handling must show `待确认` and preserve raw JD evidence when extraction/model structure is unavailable.
- Do not pass `BuildModelInput` alone into evaluation persistence: it omits evidence metadata. Do not treat model text as source fact; mark model inference and retain validation/review status.
- Existing migration numbering and architectureguard route-count checks must be updated through the established migration/guard workflow. Cross-tenant and cross-owner reads, old evaluation reads after profile mutation, request replay, and malformed/unknown model output are the highest-risk cases.

## Sources

- `docs/plans/issue-140/issues/issue-150.md`
- `docs/specs/2026-09-23-weknora-job-search-design.md`
- `docs/adr/0015-job-search-as-weknora-specialist-agent.md`, `docs/adr/0017-immutable-job-and-application-evidence.md`
- `internal/modules/career/office.go`, `model_input.go`, `opportunity.go`, `handler.go`, `office_test.go`, `opportunity_test.go`
- `apps/web/src/career/OpportunityPage.tsx`, `packages/career-core/src/contracts.ts`, `packages/api-client/src/career.ts`
