# T31 R3-F1 repair round 1 report

## Scope

Implemented the F1 relationship checks in `apps/mobile/scripts/verify-ios-scene-project.ts` and regression coverage in `apps/mobile/src/plugins/ios-xcode27.test.ts`. The checker parses the plist application scene mapping, verifies forwarding in each specific URL callback body, and checks all configurations referenced by the WeKnora native target. The fixture includes Debug and Release plus an unrelated Pods deployment setting; a drift in either app configuration is rejected.

## Verification

- `pnpm --filter @weknora/mobile exec tsx --test src/plugins/ios-xcode27.test.ts` — passed, 3/3.
- `pnpm --filter @weknora/mobile typecheck` — passed.
- `pnpm --filter @weknora/mobile test` — passed, 294 total; 280 passed, 14 skipped (opt-in deployment credentials unavailable), 0 failed.
- `pnpm --filter @weknora/mobile exec tsx scripts/verify-ios-scene-project.ts ios` — passed against the existing ignored SDK57 generated tree.
- `git diff --check -- apps/mobile/scripts/verify-ios-scene-project.ts apps/mobile/src/plugins/ios-xcode27.test.ts` — passed.

Negative evidence: the focused fixtures reject a wrong effective scene delegate, an open-URL callback without forwarding while the universal-link callback retains the linker marker, and a single app target deployment configuration changed from 16.4 to 16.0.

## Patch and commit

- Pre-commit patch SHA-256 (`git diff --` owned production/test paths, after final fixture strengthening): `83ce89a1b084803aed8e5713808572fdd59bd933efc69c67bdc6196870bbcff4`.
- Commit: `54a7dd071fb0676d5b5689962db8c11a9a0371b5` (`fix(mobile): enforce generated scene contract relationships`).

## Limitations

The synthetic target fixture contains four referenced app configuration entries (two Debug/Release pairs) and separately verifies a single-entry drift. The actual generated SDK57 project is also checked directly by the verifier.
