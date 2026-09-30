# Task Brief — T31 R4 review repair round 3

## Authority

- Root Issue #30 / descendant Issue #31; same isolated execution worktree.
- Round-2 scoped review: `.superpowers/sdd/plan-t31-ios27/fix-task-4-r2-review.md`.
- Finding R4-R2-1 (Low): the sibling-bundle fixture used `../Beta/Alpha`, which points to a nonexistent plain directory; it did not exercise the prior false-positive path to a real sibling framework executable.
- Ruling: fix the concrete plist path and add a sibling-framework symlink case. This is a valid coverage finding. Cost if wrong: a future regression could accept a sibling framework's executable despite passing the current test.

## Owned files

- Implementation agent owns only `apps/mobile/src/ios-framework-closure.test.ts` and `.superpowers/sdd/plan-t31-ios27/fix-task-4-report.md`.
- Controller owns `.superpowers/sdd/plan-t31-ios27/fix-progress.md`, this brief, round-2 review report, and `docs/plans/issue30-sweep/plans/2026-09-29-t31-review-repairs.md`.
- Do not modify the checker, release script, generated iOS project, or retained native evidence. No native build.

## Acceptance

1. Correct the fixture so `CFBundleExecutable=../Beta.framework/Alpha` resolves to the existing regular file inside the real sibling bundle; assert the production checker rejects it with the own-bundle containment error.
2. Add a second fixture with a bundle-local `Alpha` symlink resolving to `../Beta.framework/Alpha`; assert the same checker rejects it.
3. Retain all prior cases and ensure the focused production-checker integration suite passes.
4. Run full mobile tests, typecheck, Expo install check, and `git diff --check`; report exact output/counts. No native build or changes to retained release artifact/evidence.
5. Commit only owned test/report files locally and provide commit SHA plus changed-file list.

## Verification

- Focused from `apps/mobile`: `node --import tsx --test src/ios-framework-closure.test.ts src/scripts/ios-acceptance-scripts.test.ts` — all pass.
- `pnpm --filter @weknora/mobile test` — exit 0, with environment-gated skips enumerated.
- `pnpm --filter @weknora/mobile typecheck`, `pnpm --filter @weknora/mobile exec expo install --check`, and `git diff --check` — exit 0.
- No staging/OIDC or Android acceptance claims.
