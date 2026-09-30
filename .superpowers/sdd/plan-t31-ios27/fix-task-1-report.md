# Task R1+R3 implementation report

## Changed files
- `apps/mobile/scripts/ios-release-build.sh`: added `--clean`, removed retired SDK55 plugin lifecycle claims, and invokes generated-project verification before Pods.
- `apps/mobile/scripts/verify-ios-scene-project.ts`: reusable checker for the generated scene manifest, factory provider, URL callback forwarding, Xcode deployment setting, and Podfile properties.
- `apps/mobile/src/scripts/ios-acceptance-scripts.test.ts`: pins clean prebuild and prebuild → contract → Pods → Release order.
- `apps/mobile/src/native-project-config.test.ts`: labels existing app.json/package assertions as config contract coverage.
- `apps/mobile/src/plugins/ios-xcode27.test.ts`: removes retired plugin transform tests and tests generated-project contract fixtures, including missing scene, URL callback, and wrong target.
- `.superpowers/sdd/plan-t31-ios27/fix-progress.md` and `docs/plans/issue30-sweep/plans/2026-09-29-t31-review-repairs.md`: implementation evidence/status only.

## TDD and verification evidence
- RED: `pnpm --filter @weknora/mobile exec tsx --test src/scripts/ios-acceptance-scripts.test.ts src/plugins/ios-xcode27.test.ts` failed because the release script lacked `--clean` and the verifier did not yet exist.
- PASS: `pnpm --filter @weknora/mobile exec tsx --test src/scripts/ios-acceptance-scripts.test.ts src/native-project-config.test.ts src/plugins/ios-xcode27.test.ts` — 6 passed, 0 failed.
- PASS: `pnpm --filter @weknora/mobile test` — 293 total, 279 passed, 14 skipped (existing environment/credential gated tests), 0 failed.
- PASS: `pnpm --filter @weknora/mobile typecheck`.
- PASS: `git diff --check -- apps/mobile/scripts/ios-release-build.sh apps/mobile/scripts/verify-ios-scene-project.ts apps/mobile/src/scripts/ios-acceptance-scripts.test.ts apps/mobile/src/native-project-config.test.ts apps/mobile/src/plugins/ios-xcode27.test.ts`.
- BLOCKED: attempted stale sentinel followed by `pnpm --filter @weknora/mobile exec expo prebuild -p ios --clean --no-install`; Expo config failed before generation: cannot resolve `expo-build-properties`, with resolver stack from `@expo/config-plugins@55.0.11` and `@expo/config@55.0.21`. The sentinel and generated directory created for this attempt were removed. No actual SDK57 tree was available for contract verification; no Pods or full Release build was run.
- `npx expo config --type public` attempted an npm temporary install and did not provide config evidence; do not treat it as passed.

## Review / patch evidence
- Source diff reviewed locally; generated `apps/mobile/ios` output is not staged or committed.
- Implementation-only patch SHA-256 (before report/ledger additions): `36450f9d1ed5cf9bfdf0cd1dbc36b26679ecac4a19d7009ca256679c284e19c7`.
- Commit: `ca42ee9e2`.

## Remaining risk
The fixture contract and script ordering are covered, but a successful clean SDK57 prebuild and stale-tree sentinel removal remain unverified until this worktree can resolve the declared SDK57 plugin dependencies. Independent review remains pending.

## Primary integration-worktree verification addendum (2026-09-29)
The implementation agent's first prebuild attempt ran before the shared integration worktree's ignored `node_modules` links were refreshed and resolved Expo 55. The primary then ran `pnpm install --frozen-lockfile` in this worktree; package inspection confirmed Expo `57.0.26` and `expo-build-properties 57.0.22`. Re-run evidence on the exact committed code:

- `pnpm --filter @weknora/mobile exec expo prebuild -p ios --clean --no-install` — PASS, generated SDK57 iOS project.
- `pnpm --filter @weknora/mobile exec tsx scripts/verify-ios-scene-project.ts ios` — PASS.
- Added ignored `apps/mobile/ios/STALE_SDK55_SENTINEL`, reran the clean prebuild command, verified the sentinel was removed, then reran the generated project contract — PASS. Generated output remains ignored/untracked.
- Direct generated output inspection confirmed `EXExpoAppSceneDelegate`, `ExpoReactNativeFactoryProvider`, `RCTLinkingManager` app URL/continue-user-activity forwarding and Xcode/Pod deployment target `16.4`.

The earlier prebuild failure is superseded by this successful re-run; no CocoaPods install or full canonical Release script was repeated because this check's purpose was clean regeneration/contract and prior Release build evidence exists on the implementation snapshot.
