# Issue 140 Task 1 Contract and Desk Hardening Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close all Task 1 review findings before any downstream Career client consumes the versioned API and shared Desk.

**Architecture:** Contracts own strict decoded DTOs and write inputs; API client owns versioned HTTP envelopes, endpoint decoding, and a complete transport surface for the planned Career workflows; `@weknora/career-core` owns scoped intents and safe unknown/conflict reconciliation. Persisted request IDs remain immutable; retries and revised writes are distinct operations.

**Tech Stack:** TypeScript, pnpm workspace, `tsx --test`, TypeScript strict compiler.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`; approved port design `docs/superpowers/specs/2026-09-28-issue-140-main-architecture-port-design.md`; ADR 0019; Task 1 in `docs/superpowers/plans/2026-09-28-issue-140-main-port.md`; review `docs/plans/issue-140/reviews/task-1-contracts-api-desk.md` and `.superpowers/sdd/2026-09-28-issue-140-main-port/task-1-review.md`.

## Global Constraints

- Decode and validate every external response before changing application state.
- Actor, owner, and tenant authority come from authenticated server context; reject these fields in client request and response DTOs.
- Unknown write outcomes reconcile only with the original request ID; conflicts require an explicit new intent and request ID.
- Every Career Desk operation is bound to deployment, tenant, and actor scope; old-scope responses and intents are discarded.
- Web, Mini Program, and Expo consume one platform-neutral Career Desk; it must not import platform UI/runtime packages.

## Review Focus

- Failed or malformed HTTP envelopes never become successful Career data: per-endpoint negative fixtures and receipt correlation tests.
- Nested snapshots and privacy receipts cannot smuggle invalid revision, digest, identifier, or authority fields: hostile nested DTO fixtures.
- Desk can be assembled directly from `CareerApi` for planned search/application/material/timeline/privacy workflows: compile-time adapter/assembly contract test.
- Unknown, conflict, forbidden, and applied writes have distinct transitions: same-ID lookup, no blind resend, explicit new key on rebase tests.
- Scope changes during persistence, dispatch, lookup, and reads discard results and clear only the old scope’s private intents: deferred-promise scope tests.

---

### Task 1R: Harden the versioned Career contract, API surface, and Desk protocol

**Dependency:** Task 1 implementation checkpoint `75b6343583fb91f15c165877d7d3f6fa24935fc6`; independent review report above. No downstream task may start until this task passes review.

**Owner / validator:** `implementer` / independent `reviewer`.

**Files:**
- Modify: `docs/adr/0019-career-desk-shared-cross-client-state.md` (restore the reviewed interface authority from the plan-fix source if absent; do not change its approved invariants).
- Modify: `packages/contracts/src/career/**` and tests/fixtures for every exported wire variant.
- Modify: `packages/api-client/src/career/**` and tests for strict success envelopes, request validation, endpoint paths, receipts, and Career workflow methods.
- Modify: `packages/career-core/src/**` and tests for explicit outcomes and scope lifecycle.
- Modify only as required: package exports, root test/typecheck wiring, lockfile.
- Do not modify: apps, backend modules, unrelated views, Task 0 plan/DAG, or unrelated baseline failures.

**Consumes:** `CareerRequest { method, path, headers?, body? }`, `CareerRequester(input): Promise<unknown>`, existing shared HTTP client conventions; ADR 0019 operations/invariants; review findings 1–8.

**Produces:** exported strict request/response DTO parsers for all currently planned profile, opportunity/search, evaluation, application, material/submission, timeline/reminder, and privacy/export/delete receipt variants; one explicit `{ success: true, data, requestId? }` response envelope with endpoint-required receipt correlation; `CareerApi` methods and Desk adapter with structured `CareerScope { deploymentOrigin, tenantId, actorId }`; typed applied/conflict/unknown/forbidden outcomes; immutable original intent IDs and explicit new-ID rebase.

- [ ] Add failing per-endpoint tests rejecting `success:false`, missing/extra envelope keys, malformed data, mismatched/missing required receipt IDs, and valid success envelopes.
- [ ] Add contract fixtures/tests for nonempty IDs, positive safe revisions, exact nested opportunity snapshot fields/digest, confirmed fact provenance/value shape, search/export/delete receipts, and rejection of nested tenant/owner/actor authority fields.
- [ ] Add failing API tests proving empty request IDs, nonpositive/non-safe revisions, and authority-bearing write bodies cause no transport call; test every planned endpoint has the canonical method and versioned path.
- [ ] Define the Desk remote/intent-store contracts with structured scope on every read/write/lookup/store operation; ensure `CareerApi` can be composed without an unreviewed second transport/business adapter; add a compile-time assembly test.
- [ ] Add failing Desk tests for applied, unknown, conflict, and forbidden receipts; unknown recovery performs same-ID lookup before any allowed retry; conflict cannot mutate/reuse the original key; retry/rebase creates a new key; stale revisions are rejected.
- [ ] Add deferred-promise tests for scope switch during intent persistence, dispatch, lookup, and read; verify late replies are discarded, only old-scope intents are cleared, and new-scope data is retained.
- [ ] Run new focused tests before implementation and record RED; implement the minimum code; rerun focused suites and strict TypeScript checks.
- [ ] Run `pnpm test:shared` and `pnpm typecheck:shared`; expected shared tests pass. The already-recorded `packages/views/src/chat/mermaid.ts:127,158` typecheck failure is acceptable only if unchanged and still the sole failure.
- [ ] Run `git diff --check`, create the BASE..HEAD Review Package and evidence report, and commit only the owned Task 1R scope. Task 1R remains unverified until independent Spec and quality review pass.

## Self-review

- Coverage: all four high and three medium findings in the Task 1 review have an explicit test/implementation step; the low ADR traceability finding is addressed by restoring ADR 0019.
- Interfaces: the Desk and API are frozen together through a compile-time assembly test before Tasks 2–9 consume them.
- Scope: one integrated package-boundary task is necessary because response envelopes, API methods, and Desk outcomes must agree; splitting would let downstream interfaces diverge again.

### Task 1R2: Close transport classification and Desk lifecycle findings

**Dependency:** Task 1R implementation `a1be4d51b1dbf501d14de3c17c9eaa297b86a4a9`; review `.superpowers/sdd/2026-09-28-issue-140-main-port/task-1r-review.md`. Keep Task 1 unverified and downstream blocked.

**Files:** only `packages/api-client/src/career/**`, `packages/api-client/src/client.ts` if required for observer wiring, `packages/career-core/src/**`, tests, this plan/ledger and the Task 1R2 evidence report.

**Interfaces:** `ApiError` from the production shared client; typed `CareerReceipt` outcomes; structured scope and scoped intent store from ADR 0019. Only network/timeouts and undecodable outcomes are unknown; definitive 403 and 409 responses become forbidden/conflict with correlated request ID and decoded revision where supplied.

- [ ] Add an assembled production-client → CareerApi → Desk regression for HTTP 403, 409 with revision envelope, timeout, and malformed response; assert only truly indeterminate cases remain unknown.
- [ ] Add RED tests for timeout followed by a newer `open()` and attempted `rebase`; the original request ID must remain stored and lookup-able unless an authoritative conflict receipt was observed.
- [ ] Add reordered same-scope `open`/revision-hint read tests (revision 3 resolves before revision 2); the projection never regresses and a stale command cannot be accepted.
- [ ] Remove the public non-persisting `submit` bypass, or constrain any recovery-only operation to an ID already present in the active scoped store; every new write must persist before dispatch. Test no dispatch before persistence and timeout recovery afterward.
- [ ] Handle revision-hint refresh rejections without unhandled promises and wire the production client's observer source if observation is in the published seam; test decoder/network failure and scope switch during refresh.
- [ ] Run each new regression RED before implementation, then focused Career suites, strict Career/assembly type checks, `pnpm test:shared`, accepted baseline `pnpm typecheck:shared`, and `git diff --check`. Regenerate Task 1R review package and report exact BASE/HEAD/patch hash.
- [ ] Commit implementation and evidence separately; request fresh independent review. No downstream task starts until clean review.
