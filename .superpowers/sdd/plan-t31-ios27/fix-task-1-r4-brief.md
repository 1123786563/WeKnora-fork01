# Task Brief — T31 R3 F2 repair round 4

## Facts / refs
- Issue #30 implementation authorization; reviewer F2 at `.superpowers/sdd/plan-t31-ios27/fix-task-1-r3-review.md`.
- Plan: `docs/plans/issue30-sweep/plans/2026-09-29-t31-review-repairs.md` Task R3-F2.
- Current implementation commit `c69c53c8e`; report corrected at `f4afc223a`.
- Worktree `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-cont`; inspect current HEAD and preserve shared edits.

## Ownership
- `apps/mobile/scripts/verify-ios-scene-project.ts`
- `apps/mobile/src/plugins/ios-xcode27.test.ts`
- `.superpowers/sdd/plan-t31-ios27/fix-progress.md`
- `.superpowers/sdd/plan-t31-ios27/fix-task-1-r4-report.md`

## Acceptance
- Method-level URL forwarding checks must inspect executable method body with Swift line/block comments removed. Keep quoted string behavior safe: don't let `//` inside quotes truncate code.
- Add comment-only open-URL negative fixture and comment-only universal-link negative fixture while open-URL remains correct.
- Preserve verified plist scene and all-config deployment checks.

## Work / evidence
- RED each negative fixture against current substring checker.
- Fix comment handling without dependency, run focused 5+ tests, full mobile test, typecheck, actual generated SDK57 helper and diff-check.
- Save exact patch SHA/report and local commit `fix(mobile): ignore Swift comments in URL contract checks`.
- No push/deploy/Issue mutation/subagent spawning.
