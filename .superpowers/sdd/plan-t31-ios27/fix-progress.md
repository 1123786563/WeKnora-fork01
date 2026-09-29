# SDD repair ledger — T31 review findings

- Root Issue: #30 descendants, task #31/T01 iOS27 scene lifecycle.
- Source review: `.superpowers/sdd/plan-t31-ios27/review.md`, findings R1 medium, R2 medium, R3 low.
- Plan: `docs/plans/issue30-sweep/plans/2026-09-29-t31-review-repairs.md`.
- Workspace: `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-cont`.
- BASE: `db234c5eb171f2dde7427d382b55b503a038f879`; starting integrated HEAD `62ee15032`.
- Commit policy: local commits allowed, no push/merge/deploy/Issue mutation.
- Status: all findings pending; implementation begins only after task brief/preflight.

## Task R1+R3 dispatch
- Brief: `.superpowers/sdd/plan-t31-ios27/fix-task-1-brief.md`.
- Owned files: release script, its source test, generated contract test and retired plugin test only.
- Interface: clean prebuild produces the ignored SDK57 `apps/mobile/ios` tree; contract helper consumes that directory before CocoaPods.
- Commit strategy: local commit only; no push.
- Status: ready → running; R1 and R3 implementation combined because script/test boundary overlaps.
