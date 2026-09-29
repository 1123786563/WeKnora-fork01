# OCR-R2 Task 1 Report — shell hierarchy

## Result

Updated `apps/mobile/src/app-smoke.test.tsx` to assert the rendered RootLayout hierarchy directly: SafeAreaProvider's direct child is SafeAreaView, SafeAreaView's child is the rendered Expo Router Stack, and Stack existence has a dedicated assertion message. Existing edge and header option checks remain. `apps/mobile/src/app/_layout.tsx` required no changes because it already renders the accepted hierarchy.

Review repair R1 additionally asserts that the rendered RootLayout root itself is SafeAreaProvider, excluding an unexpected wrapper above the provider.

## RED → GREEN evidence

- RED: temporarily rendered SafeAreaProvider and SafeAreaView as siblings, then ran `pnpm --filter @weknora/mobile exec tsx --test --test-name-pattern='root router stack' src/app-smoke.test.tsx` from the repository root. Exit 1 at the new assertion with `SafeAreaProvider must directly contain SafeAreaView`.
- GREEN: restored the production layout and ran `pnpm --filter @weknora/mobile exec tsx --test src/app-smoke.test.tsx` from the repository root. Exit 0; 74 tests passed, 0 failed. This file includes mobile typecheck as a subprocess, which also passed.
- R1 RED: temporarily wrapped the production SafeAreaProvider in a Fragment, then ran `pnpm --filter @weknora/mobile exec tsx --test --test-name-pattern='root router stack' src/app-smoke.test.tsx`. Exit 1 at `SafeAreaProvider must be the RootLayout root` (actual root was `Symbol(react.fragment)`). The Fragment wrapper was then removed.

## Checks

- Initial test attempt before dependencies were installed: `pnpm --filter @weknora/mobile exec tsx --test src/app-smoke.test.tsx` exited 254 (`tsx` unavailable). `pnpm --filter @weknora/mobile test -- src/app-smoke.test.tsx` likewise exited 1 because this worktree initially had no `node_modules`.
- Installed with `pnpm install --frozen-lockfile`: exit 0, lockfile unchanged.
- Final targeted smoke test: `pnpm --filter @weknora/mobile exec tsx --test src/app-smoke.test.tsx` — exit 0, 74 passed / 0 failed.
- R1 final targeted smoke test: `pnpm --filter @weknora/mobile exec tsx --test src/app-smoke.test.tsx` — exit 0, 74 passed / 0 failed; embedded mobile typecheck subprocess passed.
- `git diff --check` — exit 0.

## Commit and limits

Original implementation commit: `ad68a23a83631e336400ae8167ee4bf2307fa225`. R1 repair commit: `187a0dfdda957ac6372b4d34e58fe103502ac502`. Both commits cover only the assigned test and report. The shell production source remains unchanged. No responsive or native browser behavior was changed; this task verifies static shell structure under the existing Node smoke harness.
