# T31 R3-F1 repair round 2 report

## Scope and implementation

- Updated `apps/mobile/scripts/verify-ios-scene-project.ts` to extract each ID from the WeKnora target's `buildConfigurations` array without relying on Debug/Release comments, then fail if any referenced configuration is missing or lacks `IPHONEOS_DEPLOYMENT_TARGET = 16.4`.
- Updated `apps/mobile/src/plugins/ios-xcode27.test.ts` with Debug, Release and Staging app configurations plus an unrelated Pods deployment setting. Added negative cases for an unannotated Staging ID changed to 16.0, the application scene role mapped incorrectly while the delegate marker exists under another role, and universal-link forwarding removed while open-URL forwarding stays intact. The existing wrong open-URL fixture remains.

## TDD evidence

- RED command: `pnpm --filter @weknora/mobile exec tsx --test src/plugins/ios-xcode27.test.ts`, run after adding the Staging drift fixture and before updating the parser. Result: failed as expected; 3 passed, 1 failed. The failure showed the old parser returned no issue (`actual: ''`) for the Staging 16.0 configuration because its ID was not labeled Debug/Release in the list.
- GREEN command: `pnpm --filter @weknora/mobile exec tsx --test src/plugins/ios-xcode27.test.ts` — passed, 4/4 after parser update.

## Verification

- `pnpm --filter @weknora/mobile test` — passed: 295 total, 281 passed, 14 skipped (opt-in tests require deployment credentials), 0 failed.
- `pnpm --filter @weknora/mobile typecheck` — passed (`tsc --noEmit`).
- `pnpm --filter @weknora/mobile exec tsx scripts/verify-ios-scene-project.ts ios` — passed against the actual ignored generated SDK57 tree: `Generated iOS scene contract passed: ios`.
- `git diff --check` — passed before commit.

## Patch and commit

- Owned code/test patch SHA-256 (unified `git diff` content, before commit): `874a6972170d5078632853d4335c98064efebb73d484aee44d998b174ea78530`.
- Commit: `1eb90c6e15a96420d26931aed18cfd88521a6ea5` (`fix(mobile): validate every generated iOS app config`).
- Post-commit worktree: clean.

## Limitations

The generated SDK57 tree is ignored and was verified in place; this task did not rerun Expo prebuild or an Xcode build.
