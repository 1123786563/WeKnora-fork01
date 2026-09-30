# Task Brief — T31 R3 F1 repair round 2

## Facts and references
- Parent authorization: Issue #30; repair findings in `.superpowers/sdd/plan-t31-ios27/fix-task-1-review.md` and follow-up reviewer message.
- Plan: `docs/plans/issue30-sweep/plans/2026-09-29-t31-review-repairs.md`, Task R3-F1 repair round 2.
- Current helper/tests were initially committed in `54a7dd071`; exact corrected report at `10d7ef1a1`.
- Workspace: `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-cont`; inspect current HEAD before editing. Do not revert others' edits.
- Actual ignored SDK57 tree `apps/mobile/ios` was generated with clean prebuild and contract passed; expected app has Debug/Release today.

## Ownership
- `apps/mobile/scripts/verify-ios-scene-project.ts`
- `apps/mobile/src/plugins/ios-xcode27.test.ts`
- `.superpowers/sdd/plan-t31-ios27/fix-progress.md`
- `.superpowers/sdd/plan-t31-ios27/fix-task-1-r2-report.md`

## Acceptance
- Extract every configuration object ID from WeKnora target's `buildConfigurations` array independent of config comments/name; resolve all IDs and require each corresponding `IPHONEOS_DEPLOYMENT_TARGET` to be exactly 16.4. Missing config target/setting must fail closed.
- Fixture contains Debug, Release, Staging app configurations plus an unrelated Pods/project-level value. A Staging value changed to 16.0 must fail while Debug/Release stay 16.4.
- Negative application-scene case retains `EXExpoAppSceneDelegate` elsewhere but breaks effective `UIWindowSceneSessionRoleApplication` mapping; must fail.
- Negative universal-link case leaves open-URL forwarding intact but removes RCTLinkingManager forwarding from continue-user-activity; must fail.
- Existing wrong open-URL fixture remains.

## Steps/evidence
- Add malformed fixtures first and show RED under previous checker.
- Implement parser/checker update, focused tests, full mobile test, typecheck, actual generated-tree verifier, diff-check.
- Save exact commands/results and patch SHA in named report. Commit message `fix(mobile): validate every generated iOS app config`.
- No delegation, push, deployment, or Issue mutation.
