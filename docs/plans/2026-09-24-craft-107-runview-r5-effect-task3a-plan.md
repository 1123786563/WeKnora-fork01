# Craft #107 R5 Effect Task 3a Admitted Coordinator Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the key-only RunView allocation and unclaimed provider mutation from the new Craft runtime path before physical provider wiring.

**Architecture:** Add an admitted `craft.Task` + `RunViewEffectAuthority` coordinator path. `AllocateAdmitted` establishes the persisted generation from original Task digest/fence. Every Docker/OpenCode mutation is represented by a typed effect claim for that same generation; a missing claim, `maySend=false`, unknown send or identity mismatch returns unresolved without calling the provider. The legacy `Resolve(ctx,key)` remains solely for existing tests/compatibility and cannot be selected by production Craft assembly. This Task defines the coordinator's narrow claim protocol with a fake provider; central DI and real Moby/OpenCode adapters follow in Task3b.

**Tech Stack:** Go Craft runtime coordinator, typed effect authority, fake provider tests.

**Spec:** Approved Craft #107 T01/R5; `2026-09-24-craft-107-runview-r5-fence-plan.md` Task3; R5 effect Task2b independent PASS; R4 Task2a source independent PASS. Physical default-off remains.

## Global Constraints

- No external provider call without a durable claim immediately before it. Do not invent a receipt: success requires exact observed container/session identity; response loss or cancellation after claim becomes unknown and never sends again.
- Original `craft.Task` carries server-issued Fence and snapshot digest. Never reconstruct its admission identity from a later Run `Get` or key-only RunView row.
- No central `container.go` DI, production dial/route, R4 source/capture or T19 normal-output edits in this Task. No commit/push/stash; exact checkpoint.

## Review Focus

- Valid Task allocates via admitted authority, legacy row cannot be adopted, generation stable across exact replay; mutated query/model/input/knowledge/seed digest fails before any provider call.
- Transition-first claim fails with zero calls; claim-first cancellation/lease attempt is fenced. Unknown at Docker create/start/OpenCode create parks same intent/generation; replay observes exact identity and cannot resend.
- Claimed kind, token, Run and generation bind to the one provider call; FinishEffect records exact observed receipt or unknown.

### Task 1: Typed admitted coordinator seam and fake-provider barriers

**Depends on:** R5 effect Task2b independently reviewed PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/container/craft_runview_runtime.go` and focused tests; new narrow fake authority/provider test files if needed. No `container.go`, `craft_runtime.go`, Docker engine or application repository files.

**Consumes / produces:** `craft.Task`, `craft.RunViewEffectAuthority`, existing `CraftRunViewRuntimeProvider`; produces a typed admitted Resolve method returning a bound handle only when all claims and exact observations succeed.

- [ ] Capture preimage. RED fake tests for original Task digest/fence/stale Run, legacy key-only allocation, three effect stages with transition-first/claim-first barriers, response loss/unknown/cancel, changed receipt, replay and no second provider call.
- [ ] Implement admitted path without weakening legacy path or provider interface. Bind Begin/Finish for each external mutation with an observable receipt; if existing provider methods cannot separate Docker create and start or cannot expose exact identity, report the interface blocker and leave that stage fail-closed rather than claim an unobservable success.
- [ ] Run focused coordinator tests and race, current container package compile, exact checkpoint/patch/report. Independent reviewer checks effects against actual provider call count and Task identity.

**Acceptance / failure handling:** No physical mutation can be invoked by the admitted coordinator without a one-shot effect. Real adapter/DI and R4 quiescence remain later Tasks.
