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
