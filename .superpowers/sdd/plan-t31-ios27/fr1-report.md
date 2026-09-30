# FR1 report — loader-relative framework dependency closure

## RED → GREEN

- Workspace initially had no `node_modules`; `node --import tsx --test ...` failed with `ERR_MODULE_NOT_FOUND` for `tsx`.
- Ran `pnpm install --frozen-lockfile`: lockfile was current; pnpm linked 1,915 packages and completed successfully in 28s.
- Added production-checker fixtures, then RED focused run showed 2 failures / 16 passes: unresolved `@loader_path` and `@executable_path` dependencies and unsupported `@unknown_path` were accepted.
- Checker now resolves `@rpath` against `.app/Frameworks`, `@loader_path` against the owning Mach-O image directory, and `@executable_path` against the app executable directory. It validates canonical resolution against the named framework’s declared executable and requires the resolved target remain inside the app. Unsupported non-system framework load names fail with `UNSUPPORTED_FRAMEWORK_LOAD_PATH`; `/System/Library/Frameworks/...` remains ignored.
- Real universal binary inspection revealed `otool -L` repeats the inspected binary header per architecture. The checker now filters all such headers while keeping dependency rows from every architecture.

## Verification

- Focused from `apps/mobile`: `node --import tsx --test src/ios-framework-closure.test.ts src/scripts/ios-acceptance-scripts.test.ts` — 18/18 passed.
- Full suite: `pnpm --filter @weknora/mobile test` — 316 total, 302 passed, 14 environment/credential-gated skips, 0 failed.
- Typecheck: `pnpm --filter @weknora/mobile typecheck` — pass.
- Expo alignment: `pnpm --filter @weknora/mobile exec expo install --check` — `Dependencies are up to date`.
- Diff: `git diff --check` — pass.
- Retained Release checker command: `python3 apps/mobile/scripts/verify-ios-framework-closure.py /Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-cont/apps/mobile/ios/build/Build/Products/Release-iphonesimulator/WeKnora.app /Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-cont/apps/mobile/ios/Podfile.properties.json` — prints `FRAMEWORK_MODE=source-expo-modules` and `FRAMEWORK_CLOSURE_OK`.
- Retained app executable SHA-256 remains `8d2141d06f3b6f0d185be79dff122e7fd0d102e901d72862bed7dec200e073e4`.
- No native build or evidence changes were made.

## Changed files

- `apps/mobile/scripts/verify-ios-framework-closure.py`
- `apps/mobile/src/ios-framework-closure.test.ts`
- This report.

Commit SHA: `ed9754492113ecaf16715848d1ce371f1013dcd9`.

## FR1 repair round 2

Authority: `.superpowers/sdd/plan-t31-ios27/fr1-r2-brief.md`; addressed FR1-1 and FR1-2. Base: `d888dff4b1b43e3d701303a9fc3990b094254f32`.

- RED: with the new tests in place, focused command initially failed 2 cases: `@rpath/Missing.framework` and its trailing-slash form were ignored, and an absolute missing framework dependency whose path shared the inspected app executable as a string prefix was dropped by broad header filtering.
- The checker now diagnoses framework install names missing the executable component as `MALFORMED_FRAMEWORK_LOAD_PATH`, while unsupported non-system framework forms still fail closed. Existing platform System Framework paths continue to be skipped.
- Otool parsing now skips only full-line header shapes matching `<exact inspected binary>:` or `<exact inspected binary> (architecture <arch>):`. A dependency path merely starting with the binary path is preserved and classified.
- The integration harness emits both exact architecture headers and a missing absolute dependency sharing the app executable path prefix. Malformed install-name tests cover no trailing slash and trailing slash forms.
- Focused: `node --import tsx --test src/ios-framework-closure.test.ts src/scripts/ios-acceptance-scripts.test.ts` from `apps/mobile` — 20/20 passed.
- Full: `pnpm --filter @weknora/mobile test` — 318 total, 304 passed, 14 environment/credential-gated skips, 0 failures.
- `pnpm --filter @weknora/mobile typecheck`, `pnpm --filter @weknora/mobile exec expo install --check`, and `git diff --check` all passed.
- Retained checker command again printed `FRAMEWORK_MODE=source-expo-modules` and `FRAMEWORK_CLOSURE_OK`; app executable SHA-256 remains `8d2141d06f3b6f0d185be79dff122e7fd0d102e901d72862bed7dec200e073e4`.
- `pnpm install --frozen-lockfile` was required because the fresh isolated worktree had no `node_modules`; install exited 0 with the lockfile unchanged and 1,915 packages linked.
- No native build, evidence modification, or out-of-scope changes.
- Round-2 implementation commit: `677f5e792215d4ada267b106052fe42fd38b6515`.

## FR1 repair round 3

Authority: `.superpowers/sdd/plan-t31-ios27/fr1-r3-brief.md`; base `22195943262b560e9846a1e2d0ae714202e4acc9`.

- RED: added a positive fixture for `@rpath/Foo.framework.dylib` (plus `@rpath/libFoo.dylib`); focused test failed because substring classification reported `UNSUPPORTED_FRAMEWORK_LOAD_PATH` for the non-framework dylib.
- Checker now recognizes only a complete `<name>.framework` path component followed by slash or end of token. The framework-suffix dylib and ordinary dylib fixtures pass without weakening malformed framework bundle path failures.
- The universal fixture emits distinct dependency rows after exact arm64 and x86_64 image headers; the absolute missing path-prefix dependency is repeated after each header and still fails closed. Existing unresolved framework negative cases remain in the focused suite.
- Focused `node --import tsx --test src/ios-framework-closure.test.ts src/scripts/ios-acceptance-scripts.test.ts` from `apps/mobile`: 21/21 pass.
- Full `pnpm --filter @weknora/mobile test`: 319 total, 305 passed, 14 environment/credential-gated skips, 0 failures.
- `pnpm --filter @weknora/mobile typecheck`: pass.
- `pnpm --filter @weknora/mobile exec expo install --check`: first invocation failed transiently with `read ECONNRESET`; separate retry exited 0 with `Dependencies are up to date`.
- Retained production checker printed `FRAMEWORK_MODE=source-expo-modules` and `FRAMEWORK_CLOSURE_OK`; executable SHA-256 remains `8d2141d06f3b6f0d185be79dff122e7fd0d102e901d72862bed7dec200e073e4`.
- `git diff --check`: pass. No native build or evidence changes.
- Round-3 implementation commit: `362b474e84acaa07bb89f790cd3169f8888f7333`.
