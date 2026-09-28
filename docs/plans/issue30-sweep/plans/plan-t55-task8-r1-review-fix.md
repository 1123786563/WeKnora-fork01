# T55 Task8 Review Fix R2 — keep recovery receipt attribution consistent

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Narrow follow-up to `plan-t55-task8-review-fix.md` after R1 commit `cf99b43b4a2d5c2e1e47aa2c9d7fee09e9b71e2b`.

**Goal:** Ensure emitted receipt metadata and recovery identifiers refer to the same attempted delivery, and read failures do not inherit identifiers from a previous conflict attempt.

**Findings:** R1-F1 MEDIUM: success on a later delivery leaves commit/PR/attribution fields sourced from `firstDelivery`. R1-F2 MEDIUM: after conflict/invalid input, a later `readDelivery` failure retains prior recoveryRunId/recoveryDeliveryId.

## Global Constraints
- Work only in isolated T55 Task8 worktree at HEAD `cf99b43b4a2d5c2e1e47aa2c9d7fee09e9b71e2b`.
- Owned files only: `apps/mobile/src/delivery-integration-smoke.ts`, `apps/mobile/src/delivery-integration-smoke.test.ts`.
- Preserve recovery stop/continue semantics and avoid network/credentials/live deployment.
- Local commit authorized; independent reviewer and frontend validator follow.

## Task 1 — make evidence attribution single-delivery
**Role:** `frontend_implementer`; files exactly above.
1. RED: add final evidence mapping test with first delivered row and second pushed row carrying different commit/PR/approver/login fields; assert recovered output fields identify attempted delivery. Add conflict on first candidate then read failure on second; assert failure has no stale attempt IDs, while any state evidence is explicit.
2. GREEN: select attempted delivery as receipt source whenever there was a recovery attempt; preserve first available row only for not-needed/no-attempt reporting. On a later read failure clear or separately represent prior attempt IDs so the reported failure cannot be attributed to a different row. Do not lose actual attempted state without clear semantics.
3. REFACTOR and run focused tests, mobile typecheck/full suite if supported, `git diff --check`. Commit only the two owned files; report exact commands and SHA.

**Review focus:** No JSON evidence record may mix IDs/state/receipt values from separate delivery rows. Failed read on a later run must not imply that the old run caused the failure.
