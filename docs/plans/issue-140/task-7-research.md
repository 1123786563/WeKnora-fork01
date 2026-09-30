# T07 / #147 research report (read-only)

Date: 2026-09-24 Asia/Shanghai  
Workspace: `/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`  
Evidence read: `docs/plans/issue-140/issues/issue-147.md`, `docs/specs/2026-09-23-weknora-job-search-design.md`, `CONTEXT.md`, ADR 0015–0018, `docs/plans/issue-140/2026-09-24-issue-140-dag.md`, integrated T03/T04 code and existing temporary-document upload/parser paths.

## Verified facts

- T07 is blocked by #141/T03 and owns Career Profile Intake plus the Web profile page. Acceptance requires every education, experience, project, skill, quantified achievement, and certificate item to carry source and confirmation state; pending/dismissed items must not be consumed by evaluation or material generation; failed upload must preserve confirmed facts; edits must retain versions and provenance; irrelevant identity-document numbers must be masked before model input.
- T03 already supplies the tenant/owner gate in `internal/modules/career/handler.go`: every request rechecks authenticated user, tenant membership, and the single owner member. `internal/modules/career/office.go` persists scoped `career_profiles`, confirmed `career_facts`, append-only `career_fact_versions`, pending/resolved `career_proposals`, change events, and idempotent receipts. `History` exposes per-key versions. Existing actions are `propose`, `confirm`, `confirm_proposal`, and `dismiss`; routes are `/api/v1/career/{open,list,changes,receipt}` plus `POST /act`.
- The shared wire contract in `packages/career-core/src/contracts.ts` models source (`kind`, optional label/referenceId), proposal status, confirmation, facts, revisions, and idempotency. It has no upload, parse job, profile source document, masking, or progressive-intake fields yet.
- Existing upload path is `internal/handler/session/temporary_document.go` + `TemporaryDocumentService` (`internal/types/interfaces/temporary_document.go`). It is intentionally session-scoped, expiring, asynchronously parsed, and stores `ResourceRef`, extracted `Content`/chunks/metadata and status. Upload enforces owner session access, tenant ID, byte limit, MIME/content sniffing, executable rejection, optional SHA-256, and parser options. This path can supply reusable storage/parser behavior, but its session ownership/expiry is not an authoritative career-profile identity or version.
- T04’s Workbench artifact authorization is for fixed-version task artifacts and is not a suitable profile source contract. Do not make profile intake depend on artifact grants.

## Recommended vertical seam

1. Add a Career-owned intake/source abstraction in `internal/modules/career` (likely a source-document/intake record keyed by tenant+owner, immutable source revision, status `uploaded|processing|ready|failed`, filename/MIME/size/digest/resource reference, parser error, and extracted proposal batch). It should reference the generic file/parser service rather than duplicate byte storage or parsing. A failed new intake must never mutate current `career_facts` or confirmed history.
2. Add Office methods/transactions for `BeginIntake`/`CompleteIntake` (or equivalent) and a batch proposal operation. Every extracted field must become a proposal with `Source{Kind: resume_extraction/manual, ReferenceID: immutable intake revision}`; only the existing explicit confirmation actions promote a proposal to a fact. Batch confirmation must retain the existing expected-revision/idempotency/CAS semantics and avoid duplicate facts on replay.
3. Extend the shared Career contract with intake status, source revision metadata, progressive field categories, and proposal error/unknown state before Web consumes it. Keep confirmed facts as the only downstream read model for evaluation/material generation; expose pending proposals separately.
4. Add a purpose-aware sanitization seam before model calls. It should accept confirmed structured facts plus current purpose and omit unrelated government ID/passport/national-ID fields (and raw source content unless explicitly needed). This is a policy boundary, not a UI-only redaction; add a test fixture containing an ID-like value and assert it is absent from model input while relevant facts remain.
5. Web likely extends `apps/web/src` with a Career profile/intake route, API adapter, upload progress/status, proposal review for missing/conflicting facts, and per-item confirm/dismiss actions. Reuse the existing request/upload progress patterns (`frontend/src/api/knowledge-base/index.ts` and `apps/web/src/documents/upload-pipeline.ts`) only as transport/UI precedent; do not route profile uploads through a chat session.

## Security and tenant constraints

- Reuse `Handler.scope`/`ClaimSpace` for every intake, source read, proposal list, and action. Check tenant+owner at read and write time; never trust a client-supplied source reference or tenant ID. Source references must be server-issued and immutable.
- Preserve existing content validation and size/quota controls from the upload service. Parser/model failures are non-authoritative and must return a retryable failed intake; they cannot delete or replace confirmed facts.
- Raw resume content and identity numbers need stricter retention/access than the structured profile. Do not expose `ResourceRef`, raw bytes, or unredacted extraction payload in the normal profile view.

## Meaningful RED/GREEN test targets and fixtures

- Go Career Office/handler contract tests: intake success creates only pending proposals; incomplete and conflicting sample resume fields remain pending; confirm/dismiss changes proposal state; dismissed/pending items are absent from a confirmed-facts consumer; replay is idempotent; stale revision conflicts; cross-tenant/other-owner reads fail.
- Failure/version tests: parser/upload failure leaves prior confirmed fact and history byte-for-byte unchanged; successful replacement creates a new intake/source revision and fact history retains the old source; same key confirmed twice yields one current fact plus append-only versions.
- Source fixture: a small PDF/DOCX-like extracted fixture covering education, employment, project, skill, quantified result, certificate, missing graduation date, and conflicting employer/date claims. Include source reference/digest and an ID-like field (for example national ID/passport) for redaction assertions. A deterministic parser stub is preferable to a live model.
- Sanitization tests: purpose-specific model payload contains only confirmed facts relevant to that purpose; pending/dismissed facts and unrelated ID fields are absent; source provenance remains attached to retained facts.
- Web tests: upload progress and processing/failure/retry state; review list shows source and confirmation status; confirm/dismiss updates view; failed replacement keeps existing confirmed profile; stale revision response is surfaced and refreshed. The issue’s browser acceptance should use the missing/conflicting fixture.

## Unresolved decisions / dependency gate

- Decide whether Career intake calls `TemporaryDocumentService` directly through a new interface or extracts a general upload/parser seam. Direct reuse is lower risk, but the resulting document must be promoted/copied into an immutable Career source record before expiry.
- Decide parser output schema and field taxonomy (especially quantified achievements, certificates, conflicts, and unknown values) before freezing the shared TypeScript/Go contract. Also decide whether one upload produces one batch revision or independent per-field source revisions.
- Decide retention/deletion policy for raw resume bytes and whether manual fields use a synthetic source reference or a first-class source kind.
- T03 Web does not need to be fully verified to begin backend T07 research/contract work, but T07 Web integration should wait until T03’s Web route/API shape and TDesign prerequisite are verified. T07 must not invent a parallel Career route or bypass T03’s scope/confirmation contract. T03 backend and T04 are already integrated; T03 Web remains a scheduling/integration prerequisite for the client portion.

