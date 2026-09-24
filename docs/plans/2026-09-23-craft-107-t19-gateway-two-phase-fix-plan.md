# T19 model gateway two-phase response correction

> **For Codex:** Execute this bounded SDD fix with RED→GREEN→REFACTOR, an exact uncommitted Review Package, independent Spec/quality review, and targeted revalidation. Do not claim the separate activity-ID finding is solved.

**Goal:** keep the existing journal in unresolved `intent` throughout a model response, then resolve it to `started` only after the complete bounded body is read and closed, or to `unknown` on ambiguous failure. Enforce the coordinator's 30-second initiation deadline on blocked `Do` while allowing a healthy delayed body under the longer whole-response deadline.

**Sources:** approved Craft Spec #107, T19/#138, `t19-gateway-body-fix-review.md`, `t19-gateway-high-fix-design.md`, `t19-budget-pause-service-review.md`. Integration Worktree BASE `4bcad69baf033a1310b4dce1372c8153e66adc81`; no commit authorization.

## Global Constraints

- Preserve one-shot `(grant, activity)` reservation, G4 hold, cancellation precedence, and existing `StartBinding` behavior for unrelated callers.
- The journal's `intent` and `unknown` states fence Run transitions. Resolver failure leaves `intent` held; no optimistic `started` state before body completion. Use CAS so repeated or concurrent resolution cannot promote an unknown attempt.
- Never issue a second physical model request after an ambiguous send. `X-Craft-Activity-ID` production provenance remains a separate High finding; no synthetic gateway-generated ID.
- Other agents own graph, RunView and sandbox source. Do not modify them or revert their changes.

## Review Focus

Look for a transition window where an in-flight body is marked `started`; initiator deadline not canceling blocked `Do`; deadline canceling a body after response headers; resolver write failure; absent/oversize/read/close errors; same-key retry; context race; and any post-send error mislabeled definitely-unstarted.

## Task 1 — durable two-phase attempt service

**Depends on:** T19 budget pause scoped PASS. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `internal/application/service/craft_budget.go`, `craft_budget_start_test.go` and narrowly required focused service test source only.

**Consumes:** existing prepare transaction and journal states. **Produces:** service-owned opaque attempt handle with bounded initiation context and CAS `Resolve(started|unknown|definitely_unstarted)` or equivalent two-phase API. Reuse existing prepare/resolve helpers; avoid a new migration or repository method if the existing atomic transition suffices.

1. RED tests: after prepare, Run transition remains fenced; delayed success resolves `started` once; ambiguous body failure resolves `unknown` and retains hold; same key never prepares a second call; resolver DB error keeps intent/hold; cancellation precedence stays intact.
2. Implement smallest public service seam; keep old `StartBinding` tests passing. Ensure callback/transport is outside SQL transaction and no nested transaction occurs.
3. Run focused service/repository checks and save exact hashes, patch and report.

**Acceptance:** no gap between durable prepare and final observation permits a Run transition. **Failure handling:** if existing repository CAS is inadequate, stop and request the exact repository file seam before editing it.

## Task 2 — gateway transport and body lifecycle

**Depends on:** Task 1 local GREEN; same implementation agent owns both tasks until a combined checkpoint. **Owned files:** `internal/handler/craft_model_gateway.go`, `_test.go`.

**Consumes:** two-phase service API and authenticated activity binding. **Produces:** one physical request with whole-response context, initiation timer that cancels blocked `Do` only until headers return, bounded body read/close, and final journal resolution based on full result.

1. RED tests with a real `httptest` server: stalled headers terminate at initiation deadline; headers-first delayed body survives that deadline and succeeds; inbound cancellation stops body; read/close/oversize/nil-body/(resp,error) all leave `unknown` or unresolved intent; resolver failure is not reported as success; retry has zero second provider calls.
2. Implement context callback/stop race with explicit state; resolve after body completion. Close every nonnil response body on all exit paths; record unknown usage when provider outcome is ambiguous.
3. Run focused handler/service tests, race checks where practical, `git diff --check`, and save a combined exact uncommitted checkpoint/report for independent review.

**Acceptance:** independent reviewer finds both original High body-state and Medium initiation-timeout findings resolved. **Failure handling:** a transport deadline or CAS race is a real blocker; retain fail-closed journal state and report it.
