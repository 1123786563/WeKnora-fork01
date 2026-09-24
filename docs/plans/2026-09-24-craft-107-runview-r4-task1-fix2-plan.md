# Craft #107 T01 R4 Task 1 Fix 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make message-ID adoption idempotent under same-epoch retries and prove partial draft replay plus failed-Run default-Version preservation through the production boundary.

**Architecture:** Bind adoption to the immutable delegation request and old message ID with an epoch-conditioned CAS. A losing same-request caller reloads the persisted ID; a conflicting request is denied. Extend the Execute fixture to cover partial private output and a persisted V1 default pointer during failed B.

**Tech Stack:** Go, existing RunView runtime and SQLite-backed container/service fixtures.

**Spec:** `docs/plans/2026-09-24-craft-107-runview-r4-task1-fix1-review.md`; approved Craft web Artifact Spec #107/T01; R4 version plan.

## Global Constraints

- Workspace current draft and durable Run/epoch/fence remain the only predecessor and writer authority; no previous Run directory or promoted Version fallback.
- A same-epoch retry may reuse the exact persisted message ID only for the same immutable request; a different request cannot overwrite task_json or dispatch.
- Default V1 pointer and bytes remain unchanged when B fails/stops/unknown. Partial output retry may only complete from frozen sealed bytes.
- Keep R4 runtime default-off; no H2 fixture, R5 engine, T19 or production DI edits. No commit/push; exact checkpoint and independent review.

## Review Focus

- Two concurrent same-request adopters mint different IDs but only one durable ID can win and both use that ID.
- Same tool-call/Workspace/epoch with changed request is rejected before mutation or prompt.
- CAS failure reloads and checks request, Run, Workspace, epoch and old ID; it cannot silently adopt a foreign row.
- Partially staged output with matching frozen bytes completes idempotently; changed/extra bytes reject.
- Failed B leaves persisted V1 default pointer and V1 bytes unchanged.

---

### Task 1: Adoption CAS and remaining Execute acceptance

**Depends on:** R4 Task1 Fix1 reviewed FAIL. **Owner:** `backend_implementer`. **Validator:** `backend_validator`, then independent `reviewer`.

**Owned files:** `internal/container/craft_runtime.go`, `craft_runtime_r4_seed_execute_test.go` and focused R4 test file(s). A narrow existing Version repository test helper may be used read-only; no repository production changes without separate review.

**Consumes / produces:** Durable delegation row, exact immutable request, current Run epoch and frozen draft; produces one durable message ID per request and production-boundary proof of partial replay/default preservation.

- [ ] Capture preimage hashes. RED concurrent same-request adoption and changed-request collision; assert no duplicate inner dispatch or changed task_json. RED Execute partial output retry and failed B with persisted V1 pointer/bytes fixture.
- [ ] Compare stored immutable request before adoption. Update only with current epoch and expected old prompt_message_id/task_json (or equivalent CAS). On zero rows, reload exact scoped row, verify request and epoch, reuse only its already persisted ID. Handle the two-caller race deterministically.
- [ ] Complete Execute tests for partial private output and unchanged default V1. If the runtime seam cannot observe default pointer without widening production scope, use the existing persisted Version/default service seam and document exact boundary; do not claim helper-only proof.
- [ ] Run focused normal/race and full container package, classify known H2/assembly failures, gofmt/diff checks. Save exact checkpoint/report/patch and independent review.

**Acceptance / failure handling:** Both Medium findings closed without weakening writer fence or draft identity. H2 fixture correction remains a separately owned task after this review; R4 Task2 collection remains gated.
