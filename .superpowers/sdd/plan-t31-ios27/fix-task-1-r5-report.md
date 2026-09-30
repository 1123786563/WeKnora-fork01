# T31 R3 F2 repair round 5 report

## Scope and changes

- `apps/mobile/scripts/verify-ios-scene-project.ts` now masks Swift comments and string/character literals while preserving code positions, line breaks, and braces. Signature matching and callback body extraction use that masked source; each signature must have its own balanced parameter list and `-> Bool {` before a body is accepted. Forwarding requires the executable `RCTLinkingManager.application(` token sequence inside the respective body.
- The lexical mask handles line and nested block comments, regular and multiline strings, raw string hash delimiters, escapes, and single-quoted literals. Unterminated block comments or literals fail closed for both URL callback checks. It is bounded to the generated AppDelegate syntax and is not a full Swift parser.
- `apps/mobile/src/plugins/ios-xcode27.test.ts` adds negative fixtures for string-only forwarding in each callback and for a commented open-URL signature before a valid universal-link override. A positive fixture keeps `https://`, braces in quoted text, multiline strings, and nested comments from hiding a real call; removing that real call fails. Prior comment-only callback cases remain.

## TDD and verification evidence

- RED: after the first two regression fixtures and before the checker change, `pnpm --dir apps/mobile exec tsx --test src/plugins/ios-xcode27.test.ts` returned 6 passed, 2 failed. Both new negative cases returned no issues instead of the expected open-URL failure.
- GREEN focused: same command, after the fix and lexical edge fixture, 9 passed, 0 failed.
- Full mobile: `pnpm --dir apps/mobile test` — 300 total, 286 passed, 0 failed, 14 opt-in live tests skipped without deployment credentials.
- Typecheck: `pnpm --dir apps/mobile typecheck` — passed (`tsc --noEmit`).
- Actual generated SDK57 project: `pnpm --dir apps/mobile exec tsx scripts/verify-ios-scene-project.ts ios` — `Generated iOS scene contract passed: ios`.
- Source diff check: `git diff --check` — passed before commit.

## Patch and commit

- Exact source/test patch SHA256 before report/ledger edits: `82fd31a20f95192bae15fc194ea8dcc41e086f11d64c7bde3098c1a2f3fd8d0f`.
- Implementation commit: `3060a67a952959561b4e453e21dbe2d1feb18d55` (`fix(mobile): match executable iOS URL forwarding`).

## Limits

- The checker validates the generated AppDelegate contract through a bounded lexical scanner and declaration shape. A future Expo template with different Swift declaration syntax needs an explicit verifier update; unknown or malformed syntax fails closed.
