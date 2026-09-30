# Task 0B review fix report — round 1

## Scope

Addressed only review findings F1, F2, and F4 from `task-0b-review.md` in the owned Task Office detail and native-boundary modules. No route registry, backend, or shared composition changes. Base for this repair: `8a7d45474b54a7a446b70a80f14be5eb397c8fed`.

## Changes

- Hydration rechecks the captured opening lease after projection load and save, and before publishing/returning. Scope loss clears the cached projection and aborts the stream; `view()` fails closed.
- Stream guard requires the current lease to be the exact opening lease and active. A replaced or revoked lease clears the handle projection and aborts delivery.
- Added deferred-load, deferred-save, and live-stream-after-lease-replacement regression tests. Updated two existing revocation expectations to assert cached view invalidation.
- Native capability probing now reports `installed-untested` for detected symbols. The screen labels this state `已安装，未验证`; absent adapters remain unavailable. No capability is represented as verified/available based only on symbol presence.

## Verification

- `pnpm --filter @weknora/mobile-core exec tsx --test src/task-office/task-detail.test.ts` — passed, 50/50.
- `pnpm --filter @weknora/mobile exec tsx --test src/task-office/native-boundary/task-entry.test.ts` — passed, 4/4.
- `pnpm --filter @weknora/mobile typecheck` — passed (`tsc --noEmit`).
- `git diff --check` — passed.

## Remaining risks / boundaries

- F3 (native route registration and real iOS/Android authenticated device acceptance) remains outside this assigned repair and unresolved as recorded in the original review/validation.
- Capability states report API installation only; operational device checks remain unverified and are now presented that way.
