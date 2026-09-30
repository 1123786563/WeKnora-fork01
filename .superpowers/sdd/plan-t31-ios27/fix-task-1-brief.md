# Task Brief — T31 R1 + R3: clean SDK57 iOS release project and generated contract

## Facts and authority
- Root Issue #30 authorized end-to-end descendant work; this is a repair from independent review saved in `.superpowers/sdd/plan-t31-ios27/review.md`.
- Repair plan: `docs/plans/issue30-sweep/plans/2026-09-29-t31-review-repairs.md`; approved design: `docs/specs/2026-09-20-mobile-ai-office-design.md`, `docs/specs/2026-09-20-mobile-module-seams.md`, `docs/adr/0005-weknora-native-mobile-client.md`.
- T31 implementation at `39c1359ed`; integrated ledger indicates Release startup evidence; no SDD repair task has changed production source yet.
- Workspace: `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-cont`, BASE for full execution is `db234c5eb171f2dde7427d382b55b503a038f879`, start HEAD `2e253f9d2`.

## Scope / owned files
- `apps/mobile/scripts/ios-release-build.sh`
- `apps/mobile/src/scripts/ios-acceptance-scripts.test.ts`
- `apps/mobile/src/native-project-config.test.ts`
- `apps/mobile/scripts/verify-ios-scene-project.ts` (new focused reusable generated-project contract helper)
- `apps/mobile/src/plugins/ios-xcode27.test.ts`
- `.superpowers/sdd/plan-t31-ios27/fix-progress.md` (append task evidence only)
- `.superpowers/sdd/plan-t31-ios27/fix-task-1-report.md`
- `docs/plans/issue30-sweep/plans/2026-09-29-t31-review-repairs.md` (checkbox/status only)

## Acceptance and interfaces
- Fix reviewer R1 Medium: supported Release script must use Expo clean prebuild before pods and build, so ignored SDK55 `apps/mobile/ios` output cannot survive SDK transition. Remove comments claiming old plugin is active native lifecycle source.
- Fix reviewer R3 Low: replace/remove tests of retired `ios-xcode27` transform. Add a generated project contract helper/test for the supported SDK57 output: Info.plist scene manifest with `EXExpoAppSceneDelegate`; AppDelegate `ExpoReactNativeFactoryProvider`; app URL forwarder including `RCTLinkingManager`; Xcode project target and Podfile properties deployment target 16.4. Wire helper into release script after prebuild and before pod install; retain config tests but distinguish them from generated-contract checks.
- Tests must prove negative cases (missing scene, URL callback, wrong deployment target) fail.
- Generated native tree is ignored and must not be committed.
- Do not modify login UI (Task R2 owns it), plugin source unrelated to its test, or other Release behavior.

## TDD and verification
1. Add/adjust focused script and contract tests, observe failing baseline.
2. Implement clean prebuild and contract check.
3. Run focused test files, full mobile tests, typecheck, Expo config check, and `git diff --check` for source files.
4. Run clean Expo prebuild and generated contract. Validate stale SDK55 project is removed by checking a sentinel file placed under ignored ios/ before calling the script's prebuild step (do not run full Release script twice unless needed; avoid expensive CocoaPods redownload; report limitation if full path cannot be run).
5. Review own diff and create implementation-only patch/report path `.superpowers/sdd/plan-t31-ios27/fix-task-1-report.md` with exact commands/results and patch hash.
6. Commit locally: `test(mobile): clean and verify generated SDK57 iOS project`.

## Constraints
- No push, shared-branch merge, deploy, Issue mutation, credentials access, or generated native files in Git.
- No subagent spawning. Do not revert other shared-worktree changes.
