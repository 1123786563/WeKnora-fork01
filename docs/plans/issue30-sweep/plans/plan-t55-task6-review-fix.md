# T55 Task6 Review Fix — route-scoped recovery state and behavior evidence

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. This is a focused repair wave for reviewed Task6 only.

**Goal:** Prevent delivery receipts/errors from crossing task routes, prove the rendered recovery control invokes the correct route callback, and tighten the terminal-source evidence claim.

**Architecture:** Keep delivery presentation state keyed by taskId/runId and reject stale async read or recovery completions when route identity changes. Tie the callback to the receipt currently rendered so old receipts cannot trigger recovery. Add behavior tests that invoke the actual rendered Button and a route-bound recovery handler, including success/error transitions and an in-flight route switch. Expand terminal purity markers while documenting the scan as supplementary evidence, paired with the API-client exact-surface test.

**Tech Stack:** Expo React Native component tree test harness, node:test/tsx, mobile typecheck.

**Spec:** Issue #55 snapshot; original plan-t55.md Task6; reviewed Task6 checkpoint fd119c0add2ade8156f76f6d2267ccaad38629ce; findings in Task6 independent review report.

## Global Constraints

- No API-client/mobile-core or Go changes.
- Keep TaskDetailScreen fail-closed when recovery callback is absent.
- Never show a receipt or error for a different taskId/runId.
- A late successful or failed recovery from a previous route must not mutate the current route.
- Preserve scope lease behavior and truthful pushed/unknown/delivered copy.
- Local commit authorized; independent reviewer required; no delegation.

## Review Focus

1. Delivery state is tagged to current taskId/runId and hidden immediately on route identity mismatch; effects reset current route state and async reads check identity before storing.
2. Recovery callbacks only accept the rendered current receipt's runId/deliveryId; late success/error cannot update state after route switch.
3. App smoke invokes the actual rendered Button onPress and asserts exact runId/deliveryId; success updates receipt and failure renders error without claiming completion.
4. Terminal purity checks include common direct transport/import/input markers, while test/report describe source scanning as supplemental and combine it with api-client's exact terminal method surface. Do not claim a string scan alone proves all possible transport wiring.

### Task 1: Fence UI recovery state by route and verify the actual action

**Dependencies:** Task6 checkpoint fd119c0add2ade8156f76f6d2267ccaad38629ce; original Task6 report and independent review.

**Role:** frontend_implementer; independent reviewer; targeted frontend validator after implementation.

**Worktree:** Existing isolated codex/issue30-t55-t6 worktree only, based on the reviewed implementation checkpoint.

**Owned files:**
- apps/mobile/src/app/tasks/detail.tsx
- apps/mobile/src/app-smoke.test.tsx
- apps/mobile/src/terminal-purity.test.ts
- apps/mobile/src/screens/TaskDetailScreen.tsx only if needed to pass current receipt identity into the callback.

**Report:** .superpowers/sdd/plan-t55-task6-review-fix/task-1-report.md.

**Step 1 — Add failing route-bound tests.** Extend app-smoke to locate the actual Button titled “恢复创建草稿 PR/MR” and invoke props.onPress; assert the callback receives exactly {runId: view.runId, deliveryId: receipt.deliveryId}. Add a deterministic route-bound handler test using a deferred recovery promise: begin for task/run A, switch current identity to B before resolving/rejecting A, then assert neither old receipt nor old error is written to B. Also test normal success updates the matching receipt and normal failure shows error while state remains pushed. Use channels/deferred promises, no sleeps. Add a terminal marker case for direct WebSocket/transport and input-method references that currently evade the scan; this is supplemental evidence only.

**Step 2 — Implement route-keyed presentation and stale-result fence.** Key delivery/error state by the current taskId/runId pair (or equivalent typed tuple). Render only when both receipt.taskId and receipt.runId match current route identity. Include taskId and runId in delivery-read effect dependencies; retain cancellation guard. Keep a current identity ref updated during render so async completions are ignored before effects run. In success and catch paths, update delivery/error only if the captured identity still matches; validate returned receipt identity. Ensure the TaskDetailScreen recovery callback uses the currently rendered receipt and current view.runId, and no callback is exposed for stale/mismatched receipt.

**Step 3 — Tighten terminal purity evidence.** Add checks for common interactive transport symbols/imports (for example WebSocket constructors/imports, sandbox terminal ticket routes, send/write input method names) across non-test source. Keep false positives grounded in current source. Update test name/comment to state it is a source guard, and report that this source scan supplements the Task4 API-surface test rather than claiming exhaustive proof.

**Step 4 — Verify and commit.** Run:
- pnpm exec tsx --test apps/mobile/src/terminal-purity.test.ts apps/mobile/src/app-smoke.test.tsx
- pnpm --filter @weknora/mobile test
- pnpm --filter @weknora/mobile typecheck
- git diff --check

Record any RED ordering deviation accurately. Commit only owned files as fix(mobile): fence delivery recovery state by task route.

**Acceptance:** Route change immediately hides prior receipt/error; stale read/recovery success and failure are ignored; pressing recovery invokes the correct IDs; success/error behavior is observable; terminal scan has broader coverage with an accurately bounded claim; focused/full frontend checks pass.

**Failure handling:** If the custom hook stub cannot simulate rerenders deterministically, test a pure route-bound handler extracted in detail.tsx and use mutable current-route identity with deferred promises. Do not introduce sleeps or change React test infrastructure globally. If identity semantics cannot be established, keep the test failing and report the missing seam.

## Interface and Scope Preflight

- The change consumes the existing DeliveryRecovery interface and Screen props; no dependency on future Task8 work.
- Task8 remains blocked until this task is reviewed and integrated because it tests the composition entry point.
- Owned files are distinct from Task2 Go test repair and Marketplace lifecycle work.
