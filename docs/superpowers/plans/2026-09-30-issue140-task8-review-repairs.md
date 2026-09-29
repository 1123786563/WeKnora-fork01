# Task 8 Review Repairs — Mobile server authorization boundary

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this task.

**Goal:** Resolve Task8 review findings in `task-office-integration-smoke.ts` so the live integration gate verifies server-side read and write authorization before credentials or archive operations are used.

**Architecture:** Keep client lease checks as separate defense-in-depth evidence. Add direct credential-free server probes for GET executions and POST archive using a unique nonexistent sentinel. Both must receive explicit 401/403; all other responses, redirects, and network failures stop the workflow before sign-in. Make live execution dependency-injectable for deterministic sequencing tests.

**Tech Stack:** TypeScript, Node test runner, mobile-core/API client.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`, ADR-0003/0005/0006, `CONTEXT.md`, Task8 approved plan/report, findings in `/tmp/issue140-task8-review.md`.

## Global Constraints
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue140-r2-mobile-auth`; branch `codex/issue140-r2-mobile-auth`; BASE `507427b245fccb0b1ce558fe4b5ab0e288d26bfd`.
- Preserve local commits; no push, merge, deploy, or GitHub issue mutation.
- Do not send credentials in either unauthenticated probe. Accept only 401/403; use redirect manual.
- A failed-open or unreachable probe must abort before sign-in and any archive write.

## Review Focus
- Actual unauthenticated POST reaches `/api/v1/workbench/tasks/{sentinel}/archive`, no authorization header, manual redirects.
- Only 401/403 count as rejection; 2xx, other 4xx, 3xx, or network errors fail the gate.
- Any failed server probe prevents sign-in and archive side effects.

## Task 1: Gate Task Office live integration on server read and write authorization
**Dependency:** None.
**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.
**Files:** `apps/mobile/src/task-office-integration-smoke.ts`, `apps/mobile/src/task-office-integration-smoke.test.ts`.
**Consumes / produces:** Existing live integration config/evidence contract; add explicit server write boundary evidence and injectable dependencies if needed for sequencing tests.
**Steps:**
- Add failing tests for direct archive POST request shape/status classification and run flow short-circuit on read or write fail-open/unreachable; assert sign-in, home/list, and archive calls do not occur.
- Implement direct sentinel archive probe with a unique nonexistent ID and no credentials; classify only 401/403 as rejected.
- Gate sign-in on both server boundary probes and existing client lease probes; dispose runtime on every early exit.
- Run focused mobile smoke tests, mobile typecheck, and `git diff --check`.
**Acceptance:** Live integration cannot send credentials or mutate a task unless both server unauthenticated read and write probes are explicitly rejected.

## Task 2: Independent review and verification
**Dependency:** Task 1.
**Role:** `frontend_validator`; independent `reviewer`.
**Files:** validation/review reports under `/tmp/issue140-task8-r2-*`.
**Steps:** Validate same commit, review findings and full task diff, address any valid issue with another bounded repair round.
**Acceptance:** Task8-1 and Task8-2 are addressed, no new medium/high findings, focused checks pass.
