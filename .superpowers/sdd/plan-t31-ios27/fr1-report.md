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
