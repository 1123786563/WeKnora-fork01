# Craft #107 T05 ↔ R3 H3 central assembly plan

> **For Codex:** Execute serial SDD tasks with RED → GREEN → REFACTOR, exact uncommitted checkpoints, independent Spec/quality review and backend validation. This plan does not enable production execution.

**Goal:** Wire the independently reviewed H1 accepted knowledge package and H2 exact read-only mount/final-byte verifier into the real Craft Run path, then recheck current Task/KB/document authority immediately before delegate dispatch. Remove the shared `workDir` knowledge writer from the releasable path.

**Sources:** Approved Craft Spec; `2026-09-24-craft-107-t05-r3-handoff-plan.md`; H1 Fix1 and H2 Fix3 reviews; R2 executor binding review; T05 service snapshot/dispatch recheck reviews; T08 TaskAccess reviewer findings. H3 final live A→B depends additionally on R4, which is still in progress.

## Global constraints

- Integration Worktree, no commit/push, production enablement or shared-root fallback. Preserve T01 R4 and T19 owned files.
- Server reconstructs durable actor/snapshot and Run identity. A request, model, prompt or Task owner cannot supply the knowledge root/selected KB/query. Use H1 per-Run builder and H2 final verifier with the same provider-issued material handle.
- ACL recheck is read-only and occurs after material preparation and immediately before prompt/delegate call. Revocation denies new dispatch without mutating already promoted Version evidence.
- Central `container.go` owner and final dispatch owner work serially because interface wiring is shared; no competing service singleton publisher.

## Review focus

Actual DI graph and Run loading, same actor/Run/generation through H1/H2, no global mutable publisher/workDir writer, missing ports fail closed, final current-authority recheck, zero prompt on denial, default-off until R4/live T14/T15 gates.

## Task 1 — Production DI for per-Run knowledge package

**Depends on:** H1 and H2 independently PASS; R2 binding; **reviewed T01 R5 production RunView provider/resolver and durable T05 record-store registrations**. The latter are absent in current DI (`2026-09-24-craft-107-t05-r3-h3-task1-report.md`); Task 1 is gated until they exist. **Role:** backend_implementer; validator backend_validator. **Owned files:** `internal/container/container.go` and new focused container assembly tests; no `craft_runtime.go`, T01 R4, service knowledge, or graph dispatch source. **Consumes:** `CraftKnowledgeRunViewBuilder`, durable Run store, provider material handle, current `TaskAccessChecker`, record store, production KnowledgeAccess/Search. **Produces:** `localCraftRuntime.knowledgeResolver` and `knowledgeVerifier` bound to an admitted Run and H1/H2 APIs, with missing dependency fail closed.

1. RED DI/execute tests: no runtime dial remains unavailable; configured Run A resolves only A accepted package; B cannot reuse A; absent TaskAccess/Records/provider/durable Run/actor returns before prompt; a model-supplied selection or changed snapshot cannot alter the original query/KB; shared `workDir` is never written. Existing old writer path should fail the test.
2. GREEN: replace the central shared `newCraftKnowledgeService` writer assembly with a per-Run H1 builder/factory and bind H2 verifier. Load the exact durable Run by tenant/Run ID and validate Task identity, actor, generation and snapshot before invoking H1. Keep a distinct read-only service instance for final authority if needed. Configure runtime hooks once at startup, but construct package/publisher per Run. Leave production feature dial off until end-to-end gates.
3. Run focused container/service integration and compile tests, gofmt/diff-check; save exact checkpoint/hashes and independent review. Report any DI cycle/interface gap before widening files.

**Acceptance:** configured path cannot execute without H1 accepted package and H2 final-byte check; no shared knowledge writer or fallback. **Failure:** keep runtime default-off and report wiring gap.

## Task 2 — Last-current-authority dispatch gate

**Depends on:** Task1 reviewed PASS. **Role:** backend_implementer; validator backend_validator. **Owned files:** narrow final Craft delegate dispatch source in `internal/application/service/agent_run_graph.go` or inspected equivalent and focused tests; central `container.go` only under its released owner. **Consumes:** durable actor/snapshot/Run, accepted package, `RevalidateForDispatch`. **Produces:** zero delegate/model/tool dispatch when current TaskWrite, selected KB or recorded document authority is revoked after material prep.

1. RED: revoke TaskWrite/KB/document after H2 but before delegate; forged actor, changed Run, unpublished record, snapshot mismatch or lost empty-selection TaskWrite all deny with zero dispatch. Assert no search/publish/record mutation during recheck and historical Version evidence remains immutable.
2. GREEN: place the existing read-only service recheck at the last delegate boundary after material preparation, bind actor scope from admitted Run, and fail closed for missing ports. Do not use owner as actor or rebuild package at this step.
3. Run focused normal/race and real joined ACL tests, exact checkpoint and independent review.

**Acceptance:** current authority is enforced immediately before first prompt. **Failure:** keep production default-off.

## Task 3 — Integrated live handoff

**Depends on:** Tasks1–2, R4 Task1–2, pinned Linux image, T14/T15 acceptance. **Role:** backend_validator/frontend_validator by scope, no source edits. Prove real A→B on one Task, each Run sees only its own input/knowledge and frozen current Workspace draft, read-only mounts reject writes, failed B leaves V1 default, and new Version is promoted only by T15 four checks. Save exact command/evidence and full review/OCR coverage before any release claim.
