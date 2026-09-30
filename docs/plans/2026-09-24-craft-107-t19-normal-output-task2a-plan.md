# Craft #107 T19 Normal Output Task 2a Durable Store Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Provide the concrete operation-scoped durable `Append/Seal` sink and bounded cursor reader needed by the reviewed one-attach provider, independently of live callbacks and application S2 coordination.

**Architecture:** A store owned by the application/repository records `(tenant, Task, Run, activity, container, exec, stream, sequence, bytes)` under immutable receipt identity. Each append transaction validates scope, quota and seal state, allocates one monotonic sequence and commits bytes before returning. `Seal` is prompt and linearizable against append; late append fails without mutation. A separate cursor reader projects only committed chunks. The provider receives a narrow adapter, never an arbitrary function callback.

**Tech Stack:** Go/Gorm, paired PostgreSQL/SQLite migrations, race/transaction tests.

**Spec:** `docs/plans/2026-09-24-craft-107-t19-normal-output-plan.md` Task2; `docs/plans/2026-09-24-craft-107-t19-normal-output-fix3-task1-review.md`; approved Craft #107 T19.

## Global Constraints

- No direct caller callback in the provider. Reader polling/cursor behavior cannot block Docker append/seal or change transport result.
- A store error, quota overflow, truncation or lost stream is explicit partial/unavailable evidence; no empty-string false complete. Raw stdin/output is never logged.
- Strict tenant/Task/Run/activity/exact receipt scope and retention. Same sequence replay is byte-identical or conflicts; no cross-tenant reads.
- Reserve SQLite `000124`/PG `000203` for this store, disjoint from R5 `120/121` and R4 `122/123`. No commit, routing or Docker run in Task2a.

## Review Focus

- Sealing racing with a blocked append is bounded and linearizable; after Seal returns, later unblocked append cannot commit.
- Monotonic stdout/stderr interleave and cursor reconnect are stable, with explicit gap/error states.
- Quota, byte/hash mismatch, duplicate activity/receipt and foreign scope fail closed.

---

### Task 1: Implement durable chunk append/seal and cursor read

**Depends on:** normal-output provider Fix3 independently reviewed scoped PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `migrations/sqlite/000124_craft_docker_output.{up,down}.sql`, `migrations/versioned/000203_craft_docker_output.{up,down}.sql`, new `internal/application/repository/craft_docker_output.go`/tests and new `internal/application/service/craft_docker_output.go`/tests. No existing S2/Run repository or provider files.

**Consumes / produces:** Exact immutable S2 receipt/activity key and bounded output policy; produces concrete sink adapter, sealed status/byte counts and cursor reader for committed chunks.

- [ ] Capture preimage and RED SQLite/PG tests: concurrent append sequence, stdout/stderr interleave, duplicate same/different bytes, foreign tenant/Run/receipt, cap/truncation, append failure, blocked append racing Seal and late release, cursor resume/no duplicate/gap, post-seal mutation rejection.
- [ ] Add scoped rows/indexes and a transactionally allocated sequence or equivalent unique monotonic allocator. Write bytes before returning; store digest, length, stream and receipt. The adapter's `Append` respects cancellation before commit; `Seal` is prompt with an irrevocable cutoff and waits for no arbitrary caller code. Cursor returns committed chunks only and an explicit sealed/partial state.
- [ ] Run targeted repository/service tests, race, paired migration up/down/up, format/diff. Save exact checkpoint/patch/report and independent review.

**Acceptance / failure handling:** A real durable sink can satisfy provider's enforced lifetime contract; a simulated crash/restart can read the same committed sequence. Application one-send coordinator and live caller routing remain Task2b/Task3 gates.
