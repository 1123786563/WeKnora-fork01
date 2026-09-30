# Task Brief — T31 R3 F1 repair round 1

## Pointers and workspace
- Parent/root authorization: Issue #30 full implementation; this is the SDD repair for `.superpowers/sdd/plan-t31-ios27/fix-task-1-review.md` F1.
- Plan: `docs/plans/issue30-sweep/plans/2026-09-29-t31-review-repairs.md`, section Task R3-F1.
- Spec/ADR: `docs/specs/2026-09-20-mobile-ai-office-design.md`, `docs/specs/2026-09-20-mobile-module-seams.md`, `docs/adr/0005-weknora-native-mobile-client.md`.
- Integration worktree: `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-cont`; current HEAD inspect before edits. Preserve all others' changes.

## Ownership
- `apps/mobile/scripts/verify-ios-scene-project.ts`
- `apps/mobile/src/plugins/ios-xcode27.test.ts`
- `.superpowers/sdd/plan-t31-ios27/fix-progress.md`
- `.superpowers/sdd/plan-t31-ios27/fix-task-1-r1-report.md`

## Acceptance / interface
- Current helper is invoked by Release script after clean prebuild; preserve CLI interface and no new dependency.
- Parse actual generated Info.plist application scene mapping; require exact Expo delegate.
- Prove open-URL and universal-link callback methods individually forward to RCTLinkingManager.
- Check all app target configurations use deployment 16.4; fixture at least four settings, negative one at 16.0.
- Negative fixtures retain marker text in unrelated XML/method but break actual relationship and must fail.
- Test against actual ignored SDK57 output after implementation.

## Steps/evidence
- RED for each malformed fixture before correction.
- GREEN focused contract test; full `pnpm --filter @weknora/mobile test`; typecheck; `git diff --check` on owned production/test paths.
- Report exact commands/results, patch hash, commit; commit message `fix(mobile): enforce generated scene contract relationships`.
- No push/deploy/Issue mutation or subagent delegation.
