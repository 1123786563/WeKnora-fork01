# T55 Task8 Review Fix R3 — retain the actual recovery receipt and identify reported rows

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Follow-up to `plan-t55-task8-r1-review-fix.md`, checkpoint `05d21d4fa81cf6f4427c5b1e492a7c51e983440d`.

**Goal:** Preserve the receipt returned by a successful recovery and identify any row whose metadata remains in output, separately from the run whose read failed.

**Findings:** R2-F1 MEDIUM: only `state` is retained from `recover`; successful recovery drops the resulting PR/commit/attribution receipt. R2-F2 MEDIUM: after conflict then later read failure, prior candidate state/receipt remains without its own run/delivery IDs, while failureRunId identifies another run.

## Constraints
- Existing isolated T55 Task8 worktree, based on R2 commit; only `apps/mobile/src/delivery-integration-smoke.ts` and `.test.ts`.
- No live network, secrets, backend/API client production edits or other shared files. Local commit authorized.

## Task 1 (frontend_implementer)
1. RED: test a pushed candidate with no PR metadata whose recovery returns delivered receipt with a new PR URL/commit/attribution, asserting final emitted evidence reflects returned receipt and same run/delivery IDs. Test conflict then a later read failure: output must keep `failureRunId` distinct and either clear prior delivery metadata or include explicit `deliveryRunId` + `deliveryId` identifying that metadata.
2. GREEN: carry typed `DeliveryReceiptView` (or structurally equivalent complete result) through `runDeliveryRecoveryEvidence`; verify result IDs match attempted candidate before treating successful recovery as recovered, then use returned receipt. Add explicit IDs for retained row metadata. Maintain one-write bound and conflict continuation.
3. Focused tests, mobile typecheck/full suite if feasible, diff-check; local commit and report.

**Review focus:** Each state, receipt and ID in emitted JSON has an explicit matching source row; recovered evidence contains the returned external receipt, never a stale pre-recovery projection.
