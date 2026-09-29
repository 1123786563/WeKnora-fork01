# T63 Task 3 review repair — round 2

Source: independent review of round-1 repair package `.superpowers/sdd/plan-t63/task-3-fix-round1.patch`, SHA-256 `9019c7fee1dfdb869fb24c302ac9b394f77fd647bccd640973fe0435fcdc6299`. Review verdict Spec FAIL / Quality FAIL for remaining F1 window in ordinary Adopt.

## Finding F1-R2 — High: ordinary Adopt eligibility and insert are not atomic

`AgentAdoptionRepository.AdoptListing` calls `adoptListingTx` on the root DB directly. New listing/release guards and subsequent adoption writes therefore autocommit separately; concurrent Unlist/Deprecate may commit between them. Wrap the complete ordinary AdoptListing operation, guards through insertion, in one transaction and pass its transaction DB to the existing helper. Preserve `IntroduceRelease` transaction behavior and avoid nesting. Add a deterministic gated interleaving test proving either Adopt commits before lifecycle transition or loses to it, never creating a post-transition adoption. Cover unlist and/or deprecate as plan supports; no sleeps.

## Task

- Owner: backend_implementer; validator: backend_validator; independent reviewer: reviewer.
- Worktree `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t63-task3`, branch `codex/issue30-b6-t63-task3`.
- Repair base `bef94987e78084bd1008a512762a12f2defe8a3e`.
- Owned files limited to `internal/application/repository/agent_adoption.go` and its focused lifecycle repository test, unless a narrow interface/test seam is required (report before widening).
- Preserve tenant scoping and existing conflict errors. Run targeted repository race test and existing F1/F2 repo/service tests, gofmt, diff-check. Report exact commands/results; generate repair patch/hash and commit. Do not rerun unrelated broad suite.

## Review focus

No lifecycle transition may commit between eligibility guard and adoption insertion. Test must control interleaving deterministically and observe final listing/release/adoption state. Do not rely on pre-transition sequential checks or sleeps.

## Required scope amendment — proposal acceptance eligibility

The full round-1 independent review also found a second high F1 gap: `AcceptUpgradeProposal` creates a Variant, then `TransitionProposal` commits accepted state without an atomic listing/release eligibility check. The lifecycle transition can land between those operations. Extend this repair task to `internal/application/repository/agent_upgrade.go`, its repository/service tests, and only the minimal interface signature if required. In the same DB transaction as proposal state CAS, load the tenant-scoped proposal's adoption/listing and target release, acquire the same guarded row writes/locks used by Unlist/Deprecate, and fail with existing conflict semantics if no longer eligible. Preserve the accepted-variant reference behavior and do not expand to merging variant creation into this transaction (the reviewed plan explicitly allows benign orphan draft variants). Add a deterministic test that gates after Variant creation and before proposal transition, commits Unlist or Deprecate, then proves proposal remains open/not accepted. Include this path in the round-2 report/package and review focus. Update owned-files list accordingly.
