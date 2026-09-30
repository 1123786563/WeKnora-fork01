# T31 R3 F2 repair round 4 report

## Scope and changes

- Updated `apps/mobile/scripts/verify-ios-scene-project.ts` so both independently extracted AppDelegate callback bodies are checked after Swift comments are removed. The scanner ignores line comments and nested block comments while preserving quoted strings (including `https://`), multiline strings, character literals, escapes, and newlines.
- Updated `apps/mobile/src/plugins/ios-xcode27.test.ts` with a valid URL-string control and two independent negative cases: open-URL forwarding present only in a line comment, and universal-link forwarding present only in a line comment while open-URL remains valid.
- Existing scene-role, PBX deployment, and generated-project checks remain in place.

## TDD and verification evidence

- RED: after adding both comment-only cases and before changing the verifier, `pnpm --dir apps/mobile exec tsx --test src/plugins/ios-xcode27.test.ts` failed because the open-URL comment-only case returned no issues (5 passed, 1 failed).
- GREEN focused: `pnpm --dir apps/mobile exec tsx --test src/plugins/ios-xcode27.test.ts` — 6/6 passed, including quoted `https://` control and both independent rejection assertions.
- Full mobile: `pnpm --dir apps/mobile test` — 297 tests, 283 passed, 0 failed, 14 skipped because opt-in live tests lack `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`.
- Typecheck: `pnpm --dir apps/mobile typecheck` — passed (`tsc --noEmit`).
- Generated SDK57 project: `pnpm --dir apps/mobile exec tsx scripts/verify-ios-scene-project.ts ios` — passed: `Generated iOS scene contract passed: ios`.
- Diff whitespace: `git diff --check` — passed.

## Patch and commit

- Exact patch SHA256 (pre-report source/test patch): `ebfdd71646815aa8c305dfcc4c62447f29ad719688adb64cda80f708dda2f4a5`.
- Commit: `9937fa095af4ac5ec68dde959b30e9aa60cbd2a7` (`fix(mobile): ignore Swift comments in URL contract checks`).

## Risks / limits

- The scanner covers ordinary Swift line comments, nested block comments, regular and multiline quoted strings, character literals, and escaped characters. Swift raw-string delimiter variants are not parsed specially; callback forwarding markers in such literals are not expected in the generated AppDelegate contract.
