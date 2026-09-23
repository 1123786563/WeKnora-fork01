# T07 Web review fix round 2

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore a usable new-upload path after definitive HTTP rejection and preserve a known successful POST outcome if a later source-list read fails.

**Architecture:** Distinguish the upload POST's result from follow-up refreshes. Only transport uncertainty or the backend's `outcome_unknown` code retains the original File/request ID/revision for exact replay. An explicit HTTP rejection is terminal for that attempt and leaves the form ready for a new ID. Source-list refresh failure may show a separate read error but cannot change a known POST outcome.

**Tech Stack:** React, Career API client errors, TypeScript tests.

**Sources:** T07 Web Task Brief, round-1 fix plan and independent re-review of `a9e4c6984..1b7f095a8`. Code BASE `1b7f095a87780207956786ffd4d3d5cfabb1c42f`. Same isolated Web worktree, local task commits authorized, no push/merge/deploy.

## Global Constraints

- Own only `apps/web/src/career/CareerPage.tsx` and its focused test. No backend or API contract change unless verified necessary.
- Preserve the round-1 same-name protection and forbidden epoch fencing. Do not clear an ambiguous attempt based on source list metadata.
- Do not silently start a new request after a definite rejection; merely enable a deliberate new selection/upload action with a fresh request ID.

## Review Focus

- Backend `invalid_request` (400) and `idempotency_conflict` (409) do not set `uploadUnknown` or lock the form; the reason stays visible and confirmed facts remain.
- Network failure of GET `/sources` after a 201 ready upload/receipt does not convert the known success into an unknown POST. The receipt/proposals and source stay visible, and the user can refresh sources later.
- True network ambiguity on POST still retains the original exact replay tuple. Forbidden still clears all private state and fences late responses.

---

### Task 1: Classify terminal POST errors and isolate follow-up refresh

**Depends:** R1 code and review. **Owner/validator:** frontend_implementer / frontend_validator. **Files:** `apps/web/src/career/CareerPage.tsx`, `CareerPage.test.tsx`. **Consumes:** typed client error codes and backend upload outcomes. **Produces:** safe terminal failure UI and accurate known-success state.

- [ ] RED: Add page tests for 400 `invalid_request`, 409 `idempotency_conflict`, and 201 ready receipt followed by a failing `/sources` GET. Assert fresh upload controls for the two terminal rejections, retained confirmed facts, and no unknown state after known success. Keep the R1 ambiguity and forbidden tests.
- [ ] GREEN: Narrow unknown classification to transport/timeouts/`outcome_unknown` (using existing client error shape), clear selected attempt on definitive rejection while retaining useful error message, and separate refresh handling from POST catch. Ensure follow-up profile refresh failure likewise does not relabel a known POST result. Keep authorization failures fail-closed through epoch invalidation.
- [ ] REFACTOR: Run focused Career tests, `pnpm typecheck:web`, `pnpm test:web`, `pnpm build:web`, and `git diff --check`. Save RED/GREEN evidence and exact outcomes in round-2 report; commit only owned code/tests. Independent validator and separate Spec/quality reviewer approve before integration.

## Shared-file and interface preflight

The Web worktree is the only writer of this page. Backend upload response already distinguishes 201/200/202, 400, 409 and forbidden. No shared contract change is required. Integration branch holds only plans and reports while this fix executes.
