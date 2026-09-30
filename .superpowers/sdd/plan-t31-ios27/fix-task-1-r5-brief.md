# Task Brief — T31 R3 F2 repair round 5 (final)

## Authority / facts
- Issue #30 full implementation authorization; findings in `.superpowers/sdd/plan-t31-ios27/fix-task-1-r4-review.md`.
- Repair plan: `docs/plans/issue30-sweep/plans/2026-09-29-t31-review-repairs.md`, Task R3-F2 round 5.
- Integration worktree `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-cont`; inspect current HEAD and preserve all shared changes.
- Active code previously committed at `9937fa095`; reports/ledger contain full review history.

## Owned files
- `apps/mobile/scripts/verify-ios-scene-project.ts`
- `apps/mobile/src/plugins/ios-xcode27.test.ts`
- `.superpowers/sdd/plan-t31-ios27/fix-progress.md`
- `.superpowers/sdd/plan-t31-ios27/fix-task-1-r5-report.md`

## Acceptance
- Method signature matching operates on comment-masked Swift source, so comment-only signatures are not candidates.
- Call matching operates on code tokens after comments and string/character literals are masked; actual syntax `RCTLinkingManager.application(` must appear within the respective method body.
- Add a negative fixture where the true open-URL method has only `let marker = "RCTLinkingManager.application"` and no call; must fail.
- Add a negative fixture where commented open-URL signature precedes universal-link method; it must not satisfy a missing real open-URL override.
- Keep the existing comment-only callback tests and valid `https://` string control.
- No new runtime dependency, no scope changes to release script or unrelated app UI.

## Work / evidence
- Add regression fixtures; observe RED against current checker.
- Implement a bounded lexical mask that preserves braces/newlines/method structure but removes comments and literals for matching. Do not claim full Swift parsing; parse only required syntax and fail closed on unmatched signatures/comments/literals.
- Run focused native project contract tests, full mobile suite, typecheck, actual generated SDK57 project contract, source diff-check.
- Report exact tests/hash/commit and commit `fix(mobile): match executable iOS URL forwarding`.
- No delegation, push, deploy, or Issue mutation.
