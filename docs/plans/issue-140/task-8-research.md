# T08/#146 research handoff — paste JD → opportunity/snapshot

**Read-only preparation:** 2026-09-24 (Asia/Shanghai), integration worktree
`/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`.
No application code or tests were changed/run.

## Verified facts and sources

- The #146 snapshot requires: separate raw JD and extracted fields, missing fields as unknown; replay by request ID; JD prompt injection cannot grant Agent tools; Web can open evidence details. It names #141 as the prerequisite and `Career Opportunity` as owner. Source: `docs/plans/issue-140/issues/issue-146.md` (GitHub REST snapshot metadata says read 2026-09-24).
- The approved product spec treats a job opportunity as potentially having multiple source observations and a job snapshot as fixed evidence. Source updates must not rewrite a snapshot used by an assessment/application; source, acquisition time, and original requirements are retained. Raw JD is untrusted data and cannot alter Agent permissions. Sources: `docs/specs/2026-09-23-weknora-job-search-design.md` §§2–4; `CONTEXT.md` job-opportunity/job-snapshot definitions; `docs/adr/0017-immutable-job-and-application-evidence.md`.
- The DAG marks `T08/#146` dependent on `T03/#141`, owned by `internal/modules/career/`, `packages/career-core/src/`, and `apps/web/src/`; it produces opportunity, source observation, and immutable JD snapshot interfaces. Verification is Career Office contract tests plus Web E2E paste → reopen consistency. Source: `docs/plans/issue-140/2026-09-24-issue-140-dag.md`, T08 row.
- The implementation plan fixes intended files to `internal/modules/career/service/opportunity.go`, `.../opportunity_test.go`, and `apps/web/src/career/opportunity.tsx`; intent is `CareerRemote.act({kind: "import_jd", payload}, requestId, expectedRevision)`. Source: `docs/plans/2026-09-24-issue-140-implementation.md`, Task 8.
- Integrated T03 currently has a single `internal/modules/career` package (`office.go`, `handler.go`, tests), with GORM AutoMigrate models for personal space, profile/facts/proposals/changes/receipts. `Office.Act` currently accepts only `propose`, `confirm`, `confirm_proposal`, and `dismiss`; `Handler.Act` accepts the action-shaped profile payload. Routes are `/api/v1/career/open`, `/list`, `/changes`, `/receipt`, and `POST /act`. Sources: `internal/modules/career/office.go`, `handler.go`, `internal/router/routes_career.go`.
- Existing security/idempotency seam: server derives `UserID`/`TenantID` from authenticated context and rechecks a single owner member on every HTTP request; `ClaimSpace`/`requireSpace` enforce personal-space ownership. `mutate` fingerprints request input and stores scoped receipts, so same request ID/content replays and same ID/different content returns `ErrIdempotencyConflict`; revision conflicts expose current revision. Source: `internal/modules/career/handler.go`, `office.go`, `office_test.go`, `contracts_test.go`.
- Shared TS wire types are still profile-only (`CareerFact`, `CareerProposal`, profile `CareerReceipt`, profile `CareerAction`) and decode only profile receipt discriminators. Source: `packages/career-core/src/contracts.ts` and `testdata/wire-fixtures.json`.
- ADR-0015/0016 require the opportunity flow to stay inside WeKnora identity/tenant/task semantics; ADR-0018 makes Web React/TDesign the Web presentation boundary. Sources: `docs/adr/0015-job-search-as-weknora-specialist-agent.md`, `0016-one-task-per-job-application.md`, `0018-expo-tdesign-career-clients.md`.

## Exact vertical seam for T08

1. **Typed domain/wire addition:** extend the closed Career intent/receipt/view contract with `import_jd` and an opportunity result/reference. Keep raw JD (`rawText`) physically/logically distinct from normalized extracted fields (e.g. title/company/location/batch/requirements), with explicit unknown values rather than inferred defaults. Include source kind/reference and acquisition timestamp. The opportunity/snapshot identity and snapshot content must be immutable after creation.
2. **Office persistence:** add opportunity, source observation, JD snapshot, and request receipt records scoped by authenticated `(tenant,user)`. A single transaction should create the opportunity + observation + snapshot + receipt; replay returns the stored receipt and original IDs/content. Same request ID with changed raw text or payload must reject. Do not feed raw JD to tool dispatch or treat JD text as executable instructions.
3. **HTTP/Desk seam:** route the typed action through the existing Career Office handler after T03’s authenticated scope and revision checks. `CareerRemote.act` should send only the frozen `import_jd` payload and decode a typed result; `open/list` (or an opportunity-specific read) must return evidence details without exposing another tenant’s records.
4. **Web seam:** add a C “paste JD” entry reachable from the conversation result/action. Submit once, render pending/unknown/error states, then navigate to an opportunity evidence view showing original text, source/acquired time, extracted fields, and unknown markers. Reopen/refresh must read the same snapshot and raw text.

## Immutable snapshot/source requirements

- Preserve exact user-supplied raw text; do not normalize it in place or overwrite it on retries.
- Store extraction separately and mark absent fields unknown; extraction failure still persists the raw snapshot and a non-success/needs-review state, never fabricated requirements.
- Record source observation (`manual_paste` plus optional user-provided URL/label), acquisition time, and snapshot identity/hash. Later URL/source work (#149) must append a new observation/snapshot and retain the original.
- Snapshot is evidence only. Treat all embedded instructions as data; no tool capability, approval, credential, URL fetch, or Agent dispatch may be derived from JD text.
- Tie all writes to authenticated personal Career scope, `requestId`, and `expectedRevision`; no client-supplied owner/scope is authoritative.

## TDD/public seam tests to require

In `internal/modules/career/service/opportunity_test.go` (or the package location chosen by the implementer), assert:

- complete paste stores raw text separately from fields and returns stable opportunity/snapshot IDs;
- missing title/company/location/batch/requirements are explicit unknowns;
- same request ID + identical payload returns byte-equivalent receipt and creates one opportunity/snapshot; same ID + changed text is `idempotency_conflict`;
- expected revision and cross-user/tenant access are rejected;
- JD strings containing tool-like instructions, URLs, or prompt injection remain inert data and never invoke a tool/side effect;
- extraction failure preserves raw text and returns a visible non-success/needs-review result;
- reopening the evidence read returns the original raw text, fields, source, and acquisition time unchanged.

For `packages/career-core/src/`, add fixture/decode coverage for the new discriminator and reject malformed or unknown payloads. Web component/E2E coverage should paste a JD, follow the conversation result to evidence, reload/reopen, and assert exact raw text plus unknown markers; also cover duplicate submit and visible extraction failure.

## Owned files and integration boundary

Planned ownership is exactly `internal/modules/career/service/opportunity.go`, its focused test, `packages/career-core/src/contracts.ts`/fixtures as needed for the public wire, and `apps/web/src/career/opportunity.tsx` plus focused Web tests. Shared router/container registration, broad profile contract rewrites, migrations numbering, and conversation host integration need explicit integration-owner coordination. T03 must be verified/integrated before T08 starts; T09/#149 and T10/#150 consume T08’s opportunity/snapshot contract and must not invent parallel records.

## Concrete unresolved interface decisions

1. **Canonical payload shape:** exact names/types for raw text, extracted fields, unknown markers, source observation, snapshot hash, acquisition time, and result reference are absent from the current profile-only TS contract. Freeze this before T09/T10.
2. **Revision semantics:** whether importing a JD increments the shared profile revision or uses an opportunity-local revision/cursor is unspecified. Avoid silently coupling unrelated profile facts to opportunity writes.
3. **Read endpoint shape:** current `/open` returns only profile facts/proposals. Decide whether to extend a versioned Career view, add `open/list` opportunity refs, or add a dedicated evidence read; Web needs a stable route.
4. **Extraction authority:** no extractor/parser contract is specified. Decide whether T08 uses deterministic minimal extraction, a proposal/unknown-only result, or an injected parser; never make model output authoritative.
5. **Conversation integration point:** current repository has no Career Web conversation/result component. Identify the existing chat result/action registry and define the opportunity reference navigation contract without embedding JD text as executable chat/tool input.
6. **Failure status vocabulary:** `CareerReceipt` currently has only profile kinds. Freeze whether extraction failure is a receipt status/kind or an opportunity state returned by `open`; preserve raw text in either case.

## Recommendation (clearly separated from facts)

Use an additive, versioned `import_jd` receipt with a stable `opportunityId` and `snapshotId`; store `rawText`, `extracted` (fields carrying `unknown`), and `sourceObservation` in separate records/objects. Keep opportunity revision independent from profile revision, and expose a dedicated read model for evidence so future URL/source reconciliation can append observations without rewriting history. Treat manual paste as `source.kind=manual_paste`, with no fetch or agent execution path.
