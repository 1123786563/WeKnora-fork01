# T55 Task6 Review Fix Round 2 — Route-entry generation fence

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. This is a narrow repair of one independently confirmed MEDIUM finding; it does not reopen already approved Task6 repairs.

**Goal:** Prevent an in-flight delivery read or recovery callback from an earlier visit to route A from mutating state after navigation A → B → A.

**Evidence:** Task6 review-fix round 1 commit `da2ce4911ae90c9af1686b1ee6736e88ee377ad5`; independent review identified that taskId/runId equality becomes true again after returning to A. Review report is retained in the current task output; finding: MEDIUM, stale async completion across route re-entry.

**Architecture:** Assign a monotonically increasing route-entry generation for each distinct navigation entry, including revisiting the same task/run. Async read and recovery callbacks capture both route identity and generation. They may commit state only if both still match. Do not key generation only by taskId/runId, since A→B→A must produce a new token.

## Global Constraints

- Work only in existing isolated T55 Task6 worktree `codex/issue30-t55-t6`, based on reviewed commit `da2ce4911ae90c9af1686b1ee6736e88ee377ad5`.
- Owned files: `apps/mobile/src/app/tasks/detail.tsx`, `apps/mobile/src/app-smoke.test.tsx`; touch `TaskDetailScreen.tsx` only if the current callback plumbing requires it.
- No API-client, mobile-core, Go, Marketplace or migration changes.
- Preserve existing A→B route fencing, exact callback IDs, success/error behavior, scope lease, and bounded terminal-source claim.
- Local commit authorized. After implementation, independent reviewer and frontend validator are required. Do not delegate further.

## Review Focus

1. Every new route entry has a distinct generation, including A→B→A with same taskId/runId.
2. Old A success and failure completions cannot update new A state; current A completion still works.
3. Delivery-read async completions are fenced by the same generation.
4. Tests use deferred promises/barriers, not sleeps, and exercise actual route revisit behavior.

## Task 1 — Fence asynchronous results by route-entry generation

**Dependencies:** T55 Task6 round1 checkpoint `da2ce4911`; current reviewer MEDIUM finding.

**Role:** `frontend_implementer`; then independent `reviewer` and `frontend_validator`.

**Worktree:** `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep/.worktrees/issue30-b6-coordination/.worktrees/issue30-b6-t55/.worktrees/issue30-b6-t55-t6`.

**Step 1 — RED.** Add deterministic A→B→A cases with a deferred recovery promise: start A recovery, navigate to B, return to A with identical route IDs, settle old A as success and as failure in separate cases, and prove neither mutates the new A receipt/error. Also cover current-entry success or failure and a stale delivery-read result. Run the focused tests to show the route-entry regression before implementation.

**Step 2 — Implement.** Create a route-entry token/generation that changes on every navigation transition, including same-ID re-entry. Capture it in delivery reads and recovery handlers. Before writing receipt or error, require current route identity and current generation to match the captured values. Ensure ref/effect timing cannot reuse the prior generation during render or effect cleanup; use the smallest deterministic seam consistent with the router hook behavior.

**Step 3 — Verify.** Run `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx apps/mobile/src/terminal-purity.test.ts`, `pnpm --filter @weknora/mobile test`, `pnpm --filter @weknora/mobile typecheck`, the API-client terminal-surface test, and `git diff --check`. Record exact results and any optional skipped tests.

**Step 4 — Commit.** Commit only owned files with message `fix(mobile): fence recovery by route entry` and report SHA, RED/GREEN evidence, test commands, and residual limitations.

**Acceptance:** Returning to A creates a distinct async ownership generation; old A success/error/read completions are ignored; current route behavior and prior route-keyed behavior remain correct; all required frontend checks pass.

**Failure handling:** If the router harness cannot model the entry transition, extract a small typed route-generation/guard seam and test it directly plus one actual app-level deferred callback case. Do not weaken the A→B→A assertion or introduce timing sleeps.

## Interface and Scope Preflight

- The task consumes only current route IDs, existing recovery interface and local UI state; no downstream interface changes.
- Owned files do not overlap T63 lifecycle repair or T64 security work.
- T55 Task8 remains blocked until this round is reviewed and integrated.
