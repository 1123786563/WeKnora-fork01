# Task Brief — T31 R3 F1 repair round 3

## Authority and workspace
- Issue #30 full implementation authorization; scoped residual Finding F1 from `.superpowers/sdd/plan-t31-ios27/fix-task-1-r2-review.md`.
- Plan: `docs/plans/issue30-sweep/plans/2026-09-29-t31-review-repairs.md`, repair round 3.
- Worktree: `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-cont`; inspect HEAD, do not revert others' work.
- Spec refs: `docs/specs/2026-09-20-mobile-ai-office-design.md`, seams, ADR `docs/adr/0005-weknora-native-mobile-client.md`.

## Owned files
- `apps/mobile/scripts/verify-ios-scene-project.ts`
- `apps/mobile/src/plugins/ios-xcode27.test.ts`
- `.superpowers/sdd/plan-t31-ios27/fix-progress.md`
- `.superpowers/sdd/plan-t31-ios27/fix-task-1-r3-report.md`

## Acceptance
- Resolve every ID in the app target configuration list; missing referenced config must fail.
- Read deployment setting from each config's actual buildSettings dictionary after stripping PBX block comments. Do not parse matching text inside comments. Every app config must explicitly set 16.4.
- Add Staging negative fixture where actual key is removed but `/* IPHONEOS_DEPLOYMENT_TARGET = 16.4; */` stays; checker must reject missing value. Preserve existing wrong Staging value and scene/link relationship cases.
- Keep no new package dependency, generated output untracked, checker CLI stable.

## Steps/evidence
- Add negative fixture and observe RED against current helper.
- Implement parse of actual config/buildSettings blocks and comment-free keys.
- Run focused, full mobile test, typecheck, actual generated project checker and source diff-check.
- Report exact results/patch hash. Commit `fix(mobile): require explicit deployment settings` locally. No delegation, push, release, or Issue mutation.
