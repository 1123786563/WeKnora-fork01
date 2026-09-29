# OCR-R2 Task 1 — Independent Review

Scope: commit `1df6f5cb2cdfd4253941abc5b113e778248cd189..ad68a23a83631e336400ae8167ee4bf2307fa225` in the R2-1 worktree. Reviewed the changed test and task report against Task R2-1 in `docs/plans/issue30-sweep/plans/2026-09-30-t31-ocr-r2-repairs.md`, its brief, the OCR round 2 adjudication, the approved mobile spec, ADR-0005, `CONTEXT.md`, and the current RootLayout. Read-only review: no tests or OCR run.

## Findings

1. **Medium — Root parentage acceptance remains unpinned.** At `apps/mobile/src/app-smoke.test.tsx:85-96`, `descendants(root)` searches anywhere for a provider, then checks only that provider's direct child. A RootLayout that returns a Fragment or other wrapper containing the same provider/view/Stack subtree still passes. Task R2-1 explicitly requires the rendered **root** to be SafeAreaProvider; this omission leaves a structural regression outside the test's coverage. The current production `apps/mobile/src/app/_layout.tsx:6-12` does have the required root, so this is a test/spec coverage gap, not a current runtime defect. **Smallest correction:** assert that `root` is an element whose `type` is `provider.type` before checking provider → view → Stack. A deliberate wrapper-root mutation should fail that assertion.

2. **Low — Task report omits the commit identifier required by the brief.** `.superpowers/sdd/plan-t31-ios27/ocr-r2-1-report.md:20-21` says the commit includes the test and report but does not record `ad68a23a83631e336400ae8167ee4bf2307fa225`. The R2-1 brief requests the exact commit in the report. This weakens recovery/audit traceability, though Git identifies the commit. **Smallest correction:** add the commit SHA to the report's commit section.

## Verdict

**Spec compliance: incomplete for Task R2-1.** The new direct Provider → SafeAreaView check closes the valid OCR Medium sibling gap, and `assert.ok(stack, ...)` supplies the requested clear missing-Stack diagnostic. The plan's separate root-is-provider acceptance is still untested. The unchanged production layout currently follows the intended hierarchy. The approved mobile spec and ADR-0005 add no conflicting requirement to this scoped test repair.

**Code quality: one Medium test coverage finding and one Low report finding.** The assertion order avoids dereferencing `stack` before the explicit existence check, and the view → Stack, edge, and header assertions remain. No source or security regression was found in this narrow commit. The implementer reports 74 smoke tests and typecheck passing; this review did not rerun them, as directed.
