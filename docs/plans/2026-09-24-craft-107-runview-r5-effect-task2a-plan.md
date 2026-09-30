# Craft #107 T01 R5 Effect Authority Task 2a Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist one fenced, generation-bound effect intent for RunView allocation, Docker create/start and OpenCode create without enabling those effects until every Run transition honors unresolved intents.

**Architecture:** A repository claim transaction shares the authoritative Run row lock, validates original Task/Fence and snapshot digest, active writer slot, status/owner/epoch/lease, then inserts a unique `(tenant, Run, generation, kind)` intent. Claim token is server generated; exact outcome transition records finished or unknown. This Task implements the claim store and allocation contract only; the next Task wires cancellation/reclaim/terminal/slot transitions under the same lock and forbids revocation while an intent is unresolved.

**Tech Stack:** Go/Gorm, paired SQLite/PG migrations, deterministic transactional tests.

**Spec:** `docs/plans/2026-09-24-craft-107-runview-r5-fence-plan.md` Task2; `docs/plans/2026-09-24-craft-107-runview-r5-fence-task1-fix1-review.md`; approved Craft #107 T01/ADR-0008.

## Global Constraints

- No Docker/OpenCode/production route in Task2a. An unintegrated transition protocol cannot authorize physical effects; keep default-off.
- The claim transaction uses original admission digest, exact fence, session writer slot and generation; a second ordinary `Get` is not authority. Unknown remains unresolved and no replacement generation may be allocated.
- Do not edit `agent_run_lifecycle.go` while R4 capture Task2 owns it. No commit/push. SQLite121/PG200 reserved and ignored by Git; save full bytes/hashes/patch.

## Review Focus

- Transition-first claim denial and claim uniqueness/replay for exact generation/kind; changed digest or fence rejected.
- No claim for missing/deleted/unknown or expired Run, foreign actor/tenant/session, noncurrent slot, or stale generation.
- Crash after intent commit before send and after ambiguous send never produce a second claim/token.

---

### Task 1: Fenced effect claim store and allocation predicate

**Depends on:** R5 frozen identity Fix1 independently reviewed PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `migrations/sqlite/000121_craft_run_view_effect_intent.{up,down}.sql`, `migrations/versioned/000200_craft_run_view_effect_intent.{up,down}.sql`, new `internal/application/repository/craft_run_view_effect.go`/tests, narrow `internal/application/repository/agent_run.go` and `craft_run_view.go`/tests, and typed effect authority port in a new `internal/modules/craft/run_view_effect.go` if needed. No `agent_run_lifecycle.go`, container, R4 capture or T19 files.

- [ ] Capture exact preimages. RED deterministic transaction tests for stale fence/digest/slot/lease, same generation/kind duplicate, changed receipt/outcome replay and unknown, and a concurrent Run status change committed before claim. Validate the exact Task1 digest version.
- [ ] Add unique durable effect row and opaque claim token. `BeginEffect` locks Run transition row, checks all authority fields and slot, binds generation/kind and records one permission. `FinishEffect` accepts only exact token/outcome and leaves ambiguous result unresolved. Bind `CraftRunViewStore.Allocate` to the same fence/digest predicate, without calling any physical provider.
- [ ] Run focused SQLite/PG tests, migration up/down/up, race where useful, formatting/diff, exact checkpoint/patch/report and independent review.

**Acceptance / failure handling:** Claim store is correct but production remains disabled. Task2b must gate every cancellation/reclaim/terminal/slot transfer on unresolved intents before Task3 can dispatch a claimed external effect.
