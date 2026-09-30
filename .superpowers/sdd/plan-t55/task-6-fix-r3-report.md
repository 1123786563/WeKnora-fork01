# Task 6 Repair Round 3 Report

## Finding addressed

The round 2 reviewer found that a stale action after sign-out/tenant/user change put its error under the captured old identity. The route then correctly filtered that old-scope state, but the failure became invisible, especially after sign-out when the delivery section was hidden.

Stale-scope rejection now calls a separate route handler that records a generic “登录状态或活动空间已变化，请重新进入任务详情后重试” message in the Task Detail general error state under the *current* runtime identity. It does not attach or show the old delivery receipt. The same general error is used when the current recovery provider disappears while the route identity remains.

The mounted lifecycle test now invokes the stale action after sign-out, tenant switch, and same-origin/same-tenant reauthentication. After each rejection it rerenders and asserts the generic error is visible while the previous receipt remains absent. The sign-out case verifies this without any delivery section.

## Checkpoint and package

- Repair BASE: `68fae5e887f2db3ac0a1cb78fca77d515b405f85`
- Repair HEAD: `50b0901e2858a2ed6bcdc5cb065963801475f326`
- Implementation commit: `50b0901e2858a2ed6bcdc5cb065963801475f326` — `fix(mobile): surface stale delivery recovery scope errors`
- Worktree: `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-task6`
- Branch: `codex/issue30-b6-t55-task6`
- Review package: `.superpowers/sdd/plan-t55/task-6-fix-r3-review-package.patch`
- Review package SHA-256: `be7f3c340fc1c33ecb195ddb8daaf46bd7ec3b5f77b05ecd5fcacec9118533b8`

## TDD and verification

- RED: before implementation, the mounted sign-out assertion failed because the general error was `undefined`; the stale-scope error remained bound to the old route identity.
- `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx` — PASS, 73 tests, 0 failures.
- `pnpm --filter @weknora/mobile test` — PASS, 297 total, 283 passed, 14 skipped, 0 failed.
- `pnpm --filter @weknora/mobile typecheck` — PASS (`tsc --noEmit`).
- `git diff --check 68fae5e887f2db3ac0a1cb78fca77d515b405f85 50b0901e2858a2ed6bcdc5cb065963801475f326` — PASS.

## Scope

Changed only `apps/mobile/src/app/tasks/detail.tsx` and `apps/mobile/src/app-smoke.test.tsx`. `paseo.json` remains untouched. Fresh independent review is pending.
