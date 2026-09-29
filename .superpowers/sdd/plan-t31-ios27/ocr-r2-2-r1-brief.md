# SDD Brief — OCR-R2 Task 2 Review Repair Round 1

Source task: `docs/plans/issue30-sweep/plans/2026-09-30-t31-ocr-r2-repairs.md` Task R2-2.
Implementation checkpoint: `a13263252533db91abf2baab2f00360e4c893543`.
Review: `.superpowers/sdd/plan-t31-ios27/ocr-r2-2-review.md`.

One valid Low test-coverage finding remains: production code uses plistlib for `.plist` properties, and the plan acceptance requires non-object JSON or plist roots to fail with `FRAMEWORK_MODE_PROPERTIES_INVALID`, but the implementation only added a JSON array fixture. Add a valid non-object XML plist root fixture and assert nonzero status, the same coded error, and no traceback. Do not change production code unless the new fixture demonstrates a failure. Owned files: `apps/mobile/src/ios-framework-closure.test.ts` and `.superpowers/sdd/plan-t31-ios27/ocr-r2-2-report.md`. Run the focused framework test and `git diff --check`, commit locally, and report exact result/commit. No subagents.
