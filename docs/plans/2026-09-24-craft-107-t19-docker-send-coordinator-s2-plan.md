# Craft #107 T19 Docker send coordinator S2 Plan

> **For Codex:** Execute through SDD with RED → GREEN → REFACTOR, an exact uncommitted checkpoint, backend validation and independent Spec/quality review. This is a restricted Docker safety slice, not approval to enable production chargeable execution.

**Goal:** Bind the reviewed immutable Docker exec receipt and exclusive send claim to the already committed Task Budget hold, so one logical operation can authorize at most one physical `ExecStart` request and any ambiguity remains fenced.

**Architecture:** `CraftBudgetService` owns the hold and journal. The reviewed `CraftDockerSendClaimRepository` owns receipt binding and one-owner CAS. A narrow coordinator exposes separate prepare/bind/claim/resolve steps; transport remains a later S3 task. Claimed/unknown replay is observation only. No SQL transaction encloses Docker I/O.

**Tech:** Go, GORM, existing SQLite/PostgreSQL journal and commercial hold.

**Sources:** Approved Craft Spec Task Budget/recovery; `CONTEXT.md`; ADR-0004; `2026-09-23-craft-107-t19-docker-exec-recovery-design.md`; S1 plan and `2026-09-24-craft-107-t19-docker-send-claim-s1-fix1-review.md`.

## Global constraints

- No commit, push, provider enablement, chargeable Docker call, or budget-policy change. Keep E2B, Cube and create operations fail closed.
- The prepared Run revision, full tenant/Run/activity key and immutable exact Docker receipt must agree at each transition. `claimed=true` is the sole authority for a first send. A second caller never receives it.
- `intent` plus its commercial hold remains unresolved after claim or ambiguous response; `Inspect(false,0)` and 404 are never proof of no send. Unknown remains visible and fenced.
- S1's compiled integration rerun is temporarily blocked by concurrent T01 edits; rerun relevant tests on the settled version before S2 acceptance. Do not treat S1's reviewed PG/SQLite checkpoint as missing.

## Review focus

One owner receives send authority; crash at each boundary; no new exec ID or second start on replay; hold/fence preservation; bounded context cancellation; no SQL transaction during provider I/O; no backwards compatibility bypass through `StartBinding`.

## Task 1 — Server-owned lifecycle coordinator

**Depends on:** S1 reviewed persistence checkpoint; current `CraftBudgetService` journal and hold. **Role:** backend_implementer; **validator:** backend_validator; **reviewer:** independent reviewer. **Owned files:** new `internal/application/service/craft_docker_send_coordinator.go` and focused `_test.go`; narrowly `internal/application/service/craft_budget.go` only if required for a reviewed preparation seam. No repository, migration, Docker adapter, T01 Run admission, or T05 files.

**Consumes:** authenticated admitted Run/Task binding, approved grant and stable activity key, committed journal/hold, exact `DockerExecReceipt` returned by a later inert create, S1 `BindDockerExecReceipt` and `ClaimDockerExecSend`. **Produces:** opaque prepared operation identity, exact receipt-bound claim outcome, and a single-use send permission only when the repository CAS returns true.

1. RED: prepare commits intent and hold before any simulated create; crash before receipt is safe to repeat inert create; bind persists the exact receipt; two concurrent claimants yield exactly one permission; replay after claim/unknown yields no send permission; stale Run revision/tenant/Run/activity and divergent receipt fail closed; bind/claim DB failure preserves the original unresolved hold/fence; cancellation after claim cannot reauthorize. Assert physical send callback count is at most one across crash/replay simulations.
2. GREEN: factor a narrow prepare method from existing `StartBinding` if necessary, retaining its current behavior for existing callers. Bind/claim only after preparation and enforce a current Run fence plus prepared journal revision. Make send permission nonserializable and one-shot in the coordinator API; a claim committed immediately before a caller's prospective send is never cleared. Route all ambiguous postclaim errors to `unknown` while retaining the hold. No provider method is called inside a DB transaction. Do not infer `definitely_unstarted` from Docker inspect or a canceled context after claim.
3. Verify focused normal/race tests, existing budget/journal selectors on SQLite and disposable PostgreSQL if available, formatting and `git diff --check`. Record exact pre/post file hashes, task-local patch, command outputs, DB versions and failure limitations. Independent reviewer must give separate Spec/quality conclusions; backend validator repeats the crash/concurrency and hold checks on the exact checkpoint.

**Failure handling:** if the existing budget API cannot expose a committed intent/hold without also invoking chargeable transport, stop and report the minimal seam. Do not implement a callback that can send before claim. Any unresolved activity remains fenced; no automatic retry of `ExecStart`.

## Later gates

S3 must design Docker inert create, detached bounded start and output/wait semantics using this coordinator and real engine evidence. S4 wires only reviewed Docker operations and positive observation; no other sandbox provider is implicitly enabled. Full T19 still requires model egress provenance and all Issue acceptance, OCR and joined validation.
