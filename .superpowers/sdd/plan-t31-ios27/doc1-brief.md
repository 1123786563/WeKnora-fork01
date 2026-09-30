# Task Brief — DOC1: Restore the current T31 execution plan record

## Authority and finding

- Parent plan: `docs/plans/issue30-sweep/plans/2026-09-29-t31-final-review-repairs.md`, task DOC1.
- Independent finding: `.superpowers/sdd/plan-t31-ios27/final-review.md`, T31-F2 Medium.
- Exact initial HEAD: `a2c695f18`; execute in the dedicated plan-record worktree/branch after it is fast-forwarded to include this brief commit.
- The T31 ledger, task brief/report, and repair plan cite `docs/plans/issue30-sweep/plans/plan-t31-ios27.md` as the current implementation plan, but that file is absent. The older source plan is `docs/superpowers/plans/2026-09-21-t01-ios27-scene-lifecycle.md`; it specified 16.0 and predates the approved recorded 16.4 amendment.

## Worktree, ownership, and interfaces

- Execute in `/Users/wuyongjun/.paseo/worktrees/144ixsa6/t31-doc1-plan-record`, branch `codex/t31-doc1-plan-record`.
- Role: mechanical_worker. No subagents.
- Own only new `docs/plans/issue30-sweep/plans/plan-t31-ios27.md` and `.superpowers/sdd/plan-t31-ios27/doc1-report.md`. Do not rewrite the predecessor plan, findings report, current ledger, repair plan, or app code.
- Use the predecessor plan, `CONTEXT.md`, mobile product spec, ADR 0005, Issue #31 snapshot, and these execution records as inputs: `.superpowers/sdd/plan-t31-ios27/progress.md`, `task-brief.md`, `task-report.md`, `fix-progress.md`, and the R1–R4 repair briefs/reports. Where a claimed decision cannot be located in these sources, disclose uncertainty rather than inventing approval.

## Acceptance

1. Add the cited plan at exactly `docs/plans/issue30-sweep/plans/plan-t31-ios27.md`.
2. Begin with the standard Superpowers implementation-plan header and include Goal, Architecture, Tech Stack, Spec sources, Global Constraints, Review Focus, task/interface coverage, concrete verification, self-review and status/limitations.
3. Preserve the predecessor plan link and summarize original T31 acceptance: SDK57 scene lifecycle, generated project contract, URL forwarding, iOS 27 visible startup, safe-area layout, Release dependency closure, Android compatibility checks, and external auth/capability/device checks.
4. State the approved recorded amendment to Expo SDK57 scene support and iOS deployment target 16.4; do not reuse obsolete 16.0 values as current acceptance. Point to the recorded decision/evidence in T31 `progress.md`.
5. Include T31 findings/review repairs as implementation verification tasks without claiming the pending new T31-F1 fix is already complete. Maintain statuses accurately relative to the worker's base (`a2c695f18`): original T31 code and scoped R4 repairs are present; final integrated review has requested new repairs.
6. Confirm all references in `.superpowers/sdd/plan-t31-ios27/progress.md`, `task-brief.md`, `task-report.md`, and `docs/plans/issue30-sweep/plans/2026-09-29-t31-review-repairs.md` resolve after the new file is created; do not edit those references unless necessary, and if necessary report each exact adjustment.
7. Verify cited file paths, run `git diff --check`, record commands and results in `doc1-report.md`, and commit only owned plan/report files locally.
8. No auth credential, device, or external staging claims. Stop for independent documentation review; do not self-approve.

## Failure handling

- If the source records conflict on target/version/acceptance, cite the conflicting files and the resolved explicit ruling in `progress.md`; do not infer approval from the old plan.
- If exact original task steps cannot be reconstructed from durable records, preserve the predecessor link, document which accepted code/test/evidence slices are established by execution reports, and disclose the remaining provenance limitation.
