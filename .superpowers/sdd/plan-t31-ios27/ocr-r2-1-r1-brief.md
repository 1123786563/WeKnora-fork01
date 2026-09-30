# SDD Brief — OCR-R2 Task 1 Review Repair Round 1

Source task: `docs/plans/issue30-sweep/plans/2026-09-30-t31-ocr-r2-repairs.md` Task R2-1.
Implementation checkpoint: `ad68a23a83631e336400ae8167ee4bf2307fa225`.
Review: `.superpowers/sdd/plan-t31-ios27/ocr-r2-1-review.md`.

One valid Medium remains: the smoke test checks Provider → SafeAreaView → Stack, but doesn't assert SafeAreaProvider is the rendered RootLayout root. A wrapper or Fragment above Provider could pass. Add a root type assertion to the same test and make a RED reproduction with the current SafeAreaProvider wrapped in Fragment. Also add the implementation commit SHA to the R2-1 report's Commit section. Owned files only: `apps/mobile/src/app-smoke.test.tsx`, `.superpowers/sdd/plan-t31-ios27/ocr-r2-1-report.md`, and this brief/report artifacts as requested by coordinator. Preserve production code unchanged. Run the targeted smoke test and `git diff --check`; commit locally. No subagents. Return exact results and commit.
