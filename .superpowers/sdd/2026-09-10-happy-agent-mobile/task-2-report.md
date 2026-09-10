# H02 report — native shell and product host

## Scope

- Worktree: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/codex-happy-mobile`
- Source commit: `ac64b9b4677870f7b7a9eacfd0780959229717f1`
- Imported the 693 manifest entries whose targets belong to `happy-mobile/` using `git show <commit>:<source>`; additionally imported the complete 16-file `packages/happy-wire` fixed blob closure required by its package build. Root Happy `package.json` and lockfile evidence entries were not copied over the repository root.
- Added the `@weknora/mobile` workspace, `packages/happy-wire` manifest package, Expo config, Metro wasm/Preact/inline-requires fixes, and a product-only `MobileHost` boundary.

## TDD evidence

1. RED: temporarily removed `sources/weknora/platform/host.ts`; Vitest failed during collection with `Cannot find module './host'`.
2. GREEN: restored the implementation; `pnpm --filter @weknora/mobile exec vitest run sources/weknora/platform/host.test.ts` passed: 2 tests, 0 failures.

The host accepts only an explicit `http`/`https` origin, rejects empty input as `SERVER_REQUIRED`, rejects credentials/query/hash/other protocols as `INVALID_SERVER`, and normalizes the trailing slash through `URL.origin`.

## Static checks

- `pnpm --filter @weknora/mobile exec vitest run sources/weknora/platform/host.test.ts`: PASS (2/2).
- `pnpm --filter @weknora/mobile typecheck`: PENDING/FAIL due imported upstream closure and dependency state: existing `inverted-list.tsx`, `AnimatedOverlay.tsx`, and `MobileGlass.tsx` type errors, plus `@slopus/happy-wire` declarations were unavailable before the 16-file closure import.
- `pnpm --filter @slopus/happy-wire build`: PASS (TypeScript declaration check and pkgroll completed successfully) after importing the complete 16-file wire closure.
- `expo export --platform ios|android`, `expo run:ios`, `expo run:android`: PENDING; native toolchains/simulators were not available in this worktree.

## Boundary evidence

`apps/mobile/sources/app/_layout.tsx` keeps SafeArea, keyboard, gesture, theme, and navigation providers. It does not import or invoke Happy sync, tracking/analytics, purchases/billing, realtime/voice, notification registration, or remote providers. Product host creation is explicit and contains no default upstream URL. EAS project/update URLs and upstream bundle identifiers were removed from the mobile config.

## Review follow-up

- Root shell now creates `MobileHost` only from `EXPO_PUBLIC_WEKNORA_ORIGIN`, provides it with `MobileHostProvider`, and redirects an identity-less route to `/(app)/server`; the server entry remains reachable without a host.
- Happy Wire remapping is deterministic: every fixed source under `packages/happy-wire/` maps to the same relative path under `packages/happy-wire/` (16 files), while every `packages/happy-app/sources/` manifest target maps under `apps/mobile/sources/`. Web-suffixed platform files in the manifest were imported as safe source blobs and remain platform-selected by Metro; no web-only file is used as a native entry.
- Typecheck remains environment-blocked by the exact upstream errors previously listed; Expo export and native run remain pending because native toolchains/simulators are unavailable.

Latest verification: host tests pass 2/2 and `pnpm --filter @slopus/happy-wire build` passes. Mobile typecheck remains blocked by five imported Happy upstream errors: `sources/app/(app)/dev/inverted-list.tsx` lines 137/154/171 (`ListContainer` JSX type), `sources/components/AnimatedOverlay.tsx:37` (`transform` style property), and `sources/components/MobileGlass.tsx:98` (`AnimatedStyleHandle` incompatible with `ViewStyle`). Expo export and native run remain environment-blocked because iOS/Android toolchains and simulators are unavailable.

Final review follow-up: replaced the Happy `serverConfig` screen with a WeKnora-owned origin/login entry. It validates with `createMobileHost`, updates the stateful `MobileHostProvider` via `setMobileHost`, and routes to the app after saving. It performs no server probe, Happy sync, voice setting, or persistence. The provider exposes `useMobileHost` and `useSetMobileHost`; focused host tests remain 2/2 passing.

The latest mobile typecheck still exits 2 with the five imported upstream style/list errors plus four `@slopus/happy-wire` declaration resolution errors (`sources/sync/apiTypes.ts:8`, `apiVoice.ts:6`, `storageTypes.ts:2`, `typesRaw.ts:3`). Expo export and `expo run:ios`/`expo run:android` remain blocked by unavailable native toolchains/simulators.

Final cleanup: removed the 23 manifest developer-only route files under `sources/app/(app)/dev/**`, including the old `dev/index.tsx` server controls. The two web-only mobile exclusions are `sources/app/+html.tsx` and `sources/types/react-native-webrtc-web-shim.d.ts`; neither is used by native routing. Current imported fixed source count is 677 (661 Happy app files after the 23 dev files and two web-only files are excluded, plus 16 Happy Wire files), with host/config files owned by H02.
