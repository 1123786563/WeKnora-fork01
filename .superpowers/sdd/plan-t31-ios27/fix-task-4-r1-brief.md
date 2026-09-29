# Task Brief — T31 R4 review repair round 1

## Authority and ruling

- Root Issue #30 / descendant Issue #31; current worktree `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-cont`.
- Repair only findings in `.superpowers/sdd/plan-t31-ios27/fix-task-4-review.md` (the independent review of R4 commit `eb821ee9d9e6f34dba49beb2d336c7e8396ff8de`).
- **Ruling:** revise the task-brief sentence requiring a separately embedded `React.framework`. The approved Issue #31 and approved Mobile AI Office spec require a working, safe authenticated-entry surface and compatible native runtime; neither specifies React's dynamic-vs-static packaging. Expo's supported source-build options select a coherent source-linked runtime and the installed app launched. Retain closure + launch as the acceptance outcome and allow source-linked React when no embedded framework requires `React.framework`.
- **Cost if wrong:** if an unstated release policy requires a standalone React.framework artifact, this ruling would not satisfy that policy. No such requirement appears in the approved Issue, spec, or ADR. Do not describe a React.framework as embedded in source mode.

## Owned files

- Implementation agent owns: `apps/mobile/scripts/ios-release-build.sh`, new `apps/mobile/scripts/verify-ios-framework-closure.py`, `apps/mobile/src/ios-framework-closure.test.ts`, removal of unused `apps/mobile/src/ios-framework-closure.ts`, R4 evidence under `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/`, and `.superpowers/sdd/plan-t31-ios27/fix-task-4-report.md`.
- Controller owns: `.superpowers/sdd/plan-t31-ios27/fix-progress.md` and `docs/plans/issue30-sweep/plans/2026-09-29-t31-review-repairs.md`.

No other agents may edit these paths concurrently. Do not edit generated `apps/mobile/ios/`, change Expo packaging mode, or rebuild the native app in this repair round.

## Acceptance / interfaces

1. The production release script must invoke the same checked-in framework closure checker exercised by tests; remove duplicated inline Python logic and the disconnected test-only TypeScript algorithm. A standalone Python checker may use Python's standard `plistlib` to read XML or binary `Info.plist`; no third-party runtime dependency is permitted.
2. Checker input is the actual `.app` directory. It inspects the main app executable and each executable in every embedded `*.framework` using `otool -L`; missing app executable, missing framework binary, missing `otool`, or nonzero inspection status must fail closed with the file/framework named.
3. Each non-system `@rpath/<Framework>.framework/<binary>` dependency must resolve to that framework's real `CFBundleExecutable` under `.app/Frameworks`, not merely to a directory named `<Framework>.framework`. Read XML or binary framework `Info.plist` using Python's standard `plistlib`, resolve symlinks, and require the target to be a regular file.
4. Add integration tests that execute the production checker against temporary app fixtures and a fake `otool` on `PATH`. Cover: complete closure succeeds; a missing direct app-executable dependency fails; a missing framework-binary dependency fails even when its framework directory exists; and `otool` failure fails closed.
5. Persist native evidence so it survives ignored-file cleanup: retain the full outer Release build log in a compressed tracked artifact (document original and compressed SHA-256); store actual `simctl install`/`simctl launch --console` output in a tracked text artifact (not a hand-written summary); include the Release app executable SHA-256 and screenshot SHA-256 in the report/manifest. Do not change the existing T39 baseline log.
6. Do not run another native build. Re-run the checked-in checker on the exact already-built `.app`, install and launch that unchanged app on the booted iPhone 18 Pro / iOS 27 simulator, capture actual launch output and a fresh screenshot, and verify the visible sign-in root remains below the status bar. The existing build result remains bound to its recorded binary SHA.
7. Run the focused checker/script tests, full mobile suite, typecheck, `expo install --check`, and `git diff --check`; record exact commands/counts. Commit R4 repair locally only.

## Verification contract

- Focused tests execute `node --import tsx --test src/ios-framework-closure.test.ts src/scripts/ios-acceptance-scripts.test.ts` from `apps/mobile` and pass all cases.
- Full mobile suite exits 0 with all non-opt-in tests passing; list the count of credential/environment opt-in skips.
- `pnpm --filter @weknora/mobile typecheck`, `pnpm --filter @weknora/mobile exec expo install --check`, and `git diff --check` exit 0.
- Production checker on the retained Release app prints `FRAMEWORK_CLOSURE_OK` and the build mode; each negative fixture prints the actionable failure and exits nonzero.
- Fresh captured simulator output demonstrates the unchanged Release app evaluated its JS bundle without a dyld loader error; screenshot shows login content below status bar.
- Final report names this brief, repair commit, all evidence hashes, and the no-rebuild ruling. Do not claim Issue #31 staging/OIDC or Android acceptance.
