# Task8 Orchestration Gate Test Repair

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this task.

**Goal:** Close the Task8 medium review finding by proving `runTaskOfficeIntegration` itself stops before credentials, Task Office reads/writes, and runtime use when server authorization probes fail.

**Architecture:** Add a narrow dependency injection seam for the server probe and runtime factory (or equivalent testable orchestration boundary), preserving production defaults. Test failed-open/unreachable read and write paths through the public integration runner and assert sign-in, home/list/archive, and authorized API calls are absent. Ensure the runtime is disposed on every early exit.

**Tech Stack:** TypeScript, Node test runner, mobile-core.

**Spec:** mobile AI Office spec, ADR-0003/0005/0006, Task8 R2 report `/tmp/issue140-task8-r2-review.md`, prior plans `2026-09-30-issue140-task8-review-repairs.md`.

## Global Constraints
- Worktree `/Users/wuyongjun/.codex/worktrees/issue140-r2-mobile-auth`, branch `codex/issue140-r2-mobile-auth`; BASE `4d0b7fdc7521bad4d0cd7aecc254cc32ba2ee5f3`.
- Keep the user-facing integration API default behavior unchanged; injected seam may be optional test-only configuration.
- Local commit authorized; no push/merge/deploy/GitHub mutation.

## Review Focus
- Tests exercise `runTaskOfficeIntegration`, not only the standalone gate helper.
- Both read failure and write failure branches stop before credentials/sign-in and all Task Office operations.
- Runtime disposal is observable and occurs for both branches.

## Task 1: Add integration-runner injection and failure-path tests
**Dependency:** Task8 R2.
**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.
**Files:** `apps/mobile/src/task-office-integration-smoke.ts`, `apps/mobile/src/task-office-integration-smoke.test.ts`.
**Steps:**
- Add tests against the public runner with injected probes/runtime that record calls and disposal.
- Verify read failed-open/unreachable does not call write, sign-in, or backend reads/writes; verify write failed-open/unreachable does not call sign-in or backend reads/writes; assert disposal.
- Implement the smallest optional dependency seam and run focused smoke suite, mobile typecheck, `git diff --check`.
- Commit owned files and report exact evidence.
**Acceptance:** The integration-level test fails if orchestration ever proceeds after either server auth boundary is not explicitly rejected.
