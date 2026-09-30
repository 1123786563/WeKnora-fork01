# T55 Task6 Review Fix Round 3 — Fence results on navigation blur/focus

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. This is a narrow repair wave for the remaining reviewed MEDIUM finding.

**Goal:** Invalidate delivery reads and recovery requests as soon as a detail route loses navigation focus, and assign a new async ownership epoch when the retained route regains focus.

**Evidence:** Round2 checkpoint `c338e2f0b71113bc23a4a5b18431f2e7880bdaa2` has correct handler generation comparisons, but independent review and frontend validation show generation advances only on task/run prop changes. The app uses Expo Router Stack plus `router.push`; A can remain mounted and retain props while B is on top, then regain focus with the same A identity. The round2 test manually changes generations and does not prove the production component receives actual route lifecycle events.

**Architecture:** Bind the delivery route entry epoch to Expo Router / React Navigation focus lifecycle. On every focus entry, advance an epoch; on blur cleanup, invalidate the active entry immediately. Delivery read and recovery handlers capture identity+epoch and may write only while their originating focused entry is still active. Use Expo Router's focus hook if available from the already-declared `expo-router` dependency. Keep Task Office controller lifetime semantics unchanged unless the focus implementation requires a narrowly justified adjustment.

## Global Constraints

- Work only in existing isolated T55 Task6 worktree `codex/issue30-t55-t6`, based on round2 `c338e2f0b71113bc23a4a5b18431f2e7880bdaa2`.
- Owned files: `apps/mobile/src/app/tasks/detail.tsx`, `apps/mobile/src/app-smoke.test.tsx`; add no dependency unless the current project provides no existing public hook.
- No API-client/mobile-core, Go, Marketplace, migration, router layout or unrelated changes.
- Preserve A→B parameter-change fencing, exact recovery IDs, current-route behavior, error copy and the prior terminal-source evidence.
- Local commit authorized. Independent reviewer plus frontend validator required. No further delegation by implementer.

## Review Focus

1. The actual route component subscribes to focus/blur (not just taskId/runId props); focus always yields a fresh epoch and blur invalidates pending entry work.
2. Retained A instance across A blur → B focus → B blur → A focus cannot accept old A read/success/error, even though taskId/runId stay unchanged.
3. A new recovery/read started after A refocus works normally; prior A→B changes and unmount cleanups remain safe.
4. Tests exercise the mounted `TaskDetailRouteLifecycle` focus hook seam. Prefer a real navigator harness if already available; if the repo's Node test harness cannot load native navigation packages, invoke the registered focus effect callbacks (focus, blur cleanup, refocus) while rerendering the same actual component instance and explicitly document the limitation. Do not manually mutate generation state as the only source of transition evidence.

## Task 1 — Tie route-entry ownership to navigation focus

**Dependencies:** Task6 round2 checkpoint `c338e2f0b71113bc23a4a5b18431f2e7880bdaa2`; independent MEDIUM review and read-only route lifecycle research.

**Role:** `frontend_implementer`; followed by independent `reviewer` and `frontend_validator`.

**Worktree:** `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep/.worktrees/issue30-b6-coordination/.worktrees/issue30-b6-t55/.worktrees/issue30-b6-t6`.

**Step 1 — RED.** Extend the native module test stub only for Expo Router's focus hook. Render the actual `TaskDetailRouteLifecycle` with A, invoke its registered focus callback, start a deferred recovery and a deferred delivery read, invoke blur cleanup, simulate B being focused, blur B, then refocus the retained same A component instance with unchanged taskId/runId. Resolve/reject old A operations and assert no old receipt/error writes. Start a fresh focused-A operation and prove it updates. The old implementation must fail because no focus lifecycle advances/invalidate its epoch. Avoid sleeps.

**Step 2 — Implement.** Use `useFocusEffect` from the installed Expo Router public API or another existing public navigation hook. Ensure focus callback starts a unique generation and cleanup invalidates it synchronously. Update the route ref/active flag so late handlers check both identity and focused generation before every success/error/read state write. Keep hooks' callback memoization/dependencies correct; do not call hooks conditionally.

**Step 3 — Verify.** Run focused mobile app tests and terminal-purity tests, the full mobile suite, mobile typecheck, API-client terminal-surface test and `git diff --check`. If actual navigator integration cannot run in the Node harness, state that explicitly; the component-level focus callback test must still cover retained-instance blur/refocus.

**Step 4 — Commit.** Commit only owned files with message `fix(mobile): fence delivery recovery on route focus`; report SHA, exact RED/GREEN, checks, test seam and any remaining native-navigation limitation.

**Acceptance:** An in-flight result from a route entry that blurred can never update after the same route refocuses; a fresh active-entry result succeeds; the production component is wired to navigation focus lifecycle; checks pass.

**Failure handling:** If `useFocusEffect` is not exported by installed Expo Router or mock behavior cannot be made faithful, inspect the already-installed package exports and use the supported public API; do not add an unverified hook name. If the current test harness cannot model native focus, retain a failing focused regression and report the exact missing test dependency rather than claiming the handler-only test proves lifecycle safety.

## Interface and Scope Preflight

- No external interface changes; consumers of `activeDeliveryRecovery` remain unchanged.
- Files are disjoint from T63 Task3 service files and T64 Task2 repository files.
- T55 Task8 remains blocked until this round receives review and validation PASS and its checkpoint is integrated.
