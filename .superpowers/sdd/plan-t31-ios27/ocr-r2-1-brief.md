# SDD Brief — OCR-R2 Task 1 (shell hierarchy)

Task worktree BASE: `1df6f5cb2cdfd4253941abc5b113e778248cd189`
T31 source-review HEAD before plan: `860b84016c8c6c94edd8c8adcd1ca672132efd17`
Plan: `docs/plans/issue30-sweep/plans/2026-09-30-t31-ocr-r2-repairs.md` (Task R2-1)
Owned files: `apps/mobile/src/app/_layout.tsx`, `apps/mobile/src/app-smoke.test.tsx`, `.superpowers/sdd/plan-t31-ios27/ocr-r2-1-report.md`

Read the Task R2-1 brief above for exact acceptance. This is the iOS 27 safe-area login shell slice in parent Issue #30. Current layout is already correct; the valid Medium finding is only that the smoke test fails to pin Provider → SafeAreaView parentage. Also add a clear Stack existence assertion. You may move the fixed `flex: 1` to `StyleSheet.create`; if so, update the React Native stub in app-smoke.test.tsx so the imported layout still loads. No other files are yours. Apply RED → GREEN to the structural assertion, run the targeted app smoke test and diff check, and commit locally only owned files. No subagents. Write a full report with changed files, exact checks/results, commit, and remaining limits. This worktree is shared with other task worktrees; do not revert others' work.
