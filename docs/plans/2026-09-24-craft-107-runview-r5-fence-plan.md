# Craft #107 T01 R5 Fenced Material Assembly Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close R5 Task2 Fix1 review's immutable-snapshot and writer-revocation races before any RunView, Docker or OpenCode side effect can be enabled.

**Architecture:** Admission atomically stores a canonical digest of its final seeded snapshot, which is carried independently into the Craft Task. A durable per-effect intent shares the authoritative Run transition lock: cancellation, lease reclaim and slot transfer cannot revoke an already claimed one-send effect until it is resolved, while a transition that wins first prevents the effect. The coordinator uses a fresh claim for each mutating stage; no DB transaction is held across an external call.

**Tech Stack:** Go/Gorm, paired PG/SQLite migrations, transactional Run store, Docker/OpenCode fake barriers and focused real-DB tests.

**Spec:** `docs/plans/2026-09-24-craft-107-runview-r5-fence-architecture.md`, `docs/plans/2026-09-24-craft-107-runview-r5-engine-task2-fix1-review.md`, approved Craft #107 T01 and ADR-0008.

## Global Constraints

- The final admitted snapshot digest covers query/model, input manifest, knowledge selection and Workspace seed revision. The Task receives that digest at admission; comparing current Run to itself is invalid.
- An unresolved effect intent prevents writer revocation/slot transfer or makes the Run explicitly reconciling. Neither lease timeout nor a second unguarded `Get` authorizes a new external effect.
- Each Docker create/start and OpenCode create has one persisted claim and an observable exact identity. Ambiguous send is unknown, never blindly resent; no replacement generation. Keep feature gate default-off until every transition path observes the protocol.
- No commit/push or production dial enablement. Exact uncommitted Task checkpoint, targeted tests and independent Spec/quality review before downstream Task.
- Reserve migration pairs SQLite `000120`/PG `000199` for Task1 and SQLite `000121`/PG `000200` for Task2; R4 candidate/capture must use later numbers to avoid conflict.

## Review Focus

- Same Workspace with changed valid query/model/input/knowledge/seed revision rejects before allocation.
- At each `Get→Allocate→Docker create/start→OpenCode create` barrier, transition-first means zero later external calls; claim-first allows at most that one claimed effect and blocks/reconciles the transition.
- Crash before send and response loss after send keep exact intent identity and never cause a second chargeable create.
- Missing digest, legacy Run or incomplete transition integration fail closed.

---

### Task 1: Persist and propagate frozen snapshot identity

**Depends on:** R5 Fix1 review FAIL. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `migrations/sqlite/000120_craft_run_snapshot_digest.{up,down}.sql`, `migrations/versioned/000199_craft_run_snapshot_digest.{up,down}.sql`, `internal/application/repository/agent_run.go`, `internal/application/repository/agent_run_craft_seed_test.go`, `internal/modules/agentruntime/agent/runtime/contracts.go` and focused runtime propagation tests, `internal/modules/craft/contracts.go`, `internal/application/service/craft_delegate.go`, `internal/modules/agentruntime/agent/tools/craft_delegate.go` and their directly focused tests. `AgentRunStore.ClaimDriver` constructs the server-owned runtime Fence; `CraftDelegateTool.Execute` at `internal/modules/agentruntime/agent/tools/craft_delegate.go:246` is the exact Task constructor; the application registers its server-owned config at `internal/application/service/craft_delegate.go:350`. Carry the admission digest through the Fence/context rather than reading a later Run row as the Task's identity. No `container.go` or `agent_service.go` in Task1.

**Consumes / produces:** final admitted snapshot and Run row; produces versioned canonical digest persisted atomically and independently carried to `craft.Task`.

- [ ] Capture HEAD/preimage and inspect exact Task construction. RED PG/SQLite tests for same Workspace but changed valid query/model/input/knowledge/seed revision; Task digest retained from original admission must reject a changed durable snapshot. Test absent/unknown digest and legacy Run fail closed.
- [ ] Add digest/version columns with safe legacy null state. Canonicalize the **final** seeded snapshot once inside admission transaction and persist digest; expose it through typed Run and Task construction without re-deriving Task digest from a later `Get`. Recompute from persisted bytes for corruption detection.
- [ ] Run focused admission/repository/service tests and paired migration up/down/up, `gofmt -d`, diff check. Save checkpoint/patch/report and independent review.

**Acceptance / failure handling:** Task and durable Run identities agree only when both originate from the same final admitted bytes. Do not backfill ambiguous legacy rows. If no Task construction path can carry the digest, report exact interface blocker and keep R5 gated.

### Task 2: Durable one-effect claim shared with Run transitions

**Depends on:** Task1 independently reviewed PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `migrations/sqlite/000121_craft_run_view_effect_intent.{up,down}.sql`, `migrations/versioned/000200_craft_run_view_effect_intent.{up,down}.sql`, `internal/application/repository/agent_run.go`, `agent_run_lifecycle.go`, `craft_run_view.go`, new effect store files and focused tests. Coordinate any additional terminal/cancel transition file ownership before edit.

**Consumes / produces:** authoritative Run/slot lock and Task digest/fence; produces `BeginEffect/FinishEffect` and allocation bound to one intent. Cancellation/reclaim/terminal paths check unresolved intents under that same lock.

- [ ] RED transactional barrier tests for transition-first/claim-first across cancel, lease expiry/reclaim, terminal and slot transfer; one-send replay under crash/unknown. Use deterministic channels/DB locks, not sleeps.
- [ ] Add unique `(tenant, run, generation, kind)` intent and opaque token; lock/verify actor/scope/status/owner/epoch/lease/digest/slot in one transaction. Make `Allocate` take the same authority; gate every revocation/transfer on unresolved intent. Unknown intent blocks new writer until exact reconciliation. Never hold DB lock during external operation.
- [ ] Run focused SQLite/PG and migration tests, race where relevant, formatting/diff; exact checkpoint and independent review.

**Acceptance / failure handling:** All transition paths demonstrably participate. If one path is missing, keep material create disabled and report it before Task3.

### Task 3: Wire claims through coordinator and physical providers

**Depends on:** Task2 independently reviewed PASS and R4 Task2a source review PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/container/container.go`, `craft_runview_runtime.go`, provider and focused assembly/runtime tests, assigned serially after current owners release them.

**Consumes / produces:** effect authority port, admitted digest and verified generation; produces claim-before-allocate/create/start/session behavior with exact outcome recording and no resend after unknown.

- [ ] RED deterministic barrier tests at every external call, including mutated same-Workspace snapshot and missing/deleted/unknown Run with zero-call counters.
- [ ] Pass authority to coordinator; claim immediately before each mutating operation, finish or park unknown, reacquire fresh fence for the next stage. Never use a raw `Get` as the decisive dispatch gate.
- [ ] Focused/race/real Docker where applicable, exact checkpoint and review. H3 central DI and production route remain gated until this Task passes.

**Acceptance / failure handling:** If a provider lacks exact inspect/reconciliation identity, leave that effect unknown and the route disabled; do not fabricate a successful material handle.
