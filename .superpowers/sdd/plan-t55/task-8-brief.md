# Task Brief — T55 Task 8

## Source and accepted scope

Root Issue #30 → descendant Issue #55 (T25), approved source snapshot and DAG: `docs/plans/issue30-sweep/issues/issue-55.md`, `docs/plans/issue30-sweep/dag.md`; implementation plan: `docs/plans/issue30-sweep/plans/plan-t55.md`, Task 8. Read `CONTEXT.md`, relevant approved specs, and Task 5/6 implementation interfaces before editing. Task 6 is integrated and independently reviewed. Task 8 adds an opt-in real deployment recovery evidence leg only.

## Workspace and recovery

- Worktree: `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-task8`
- Branch: `codex/issue30-b6-t55-task8`
- BASE: `8af5adc06` (T55 integration HEAD after Task 6 review record)
- Commit strategy: local task commits authorized; no push, merge, deploy, publication, or Issue mutation.
- Preserve unrelated `paseo.json` if present. Do not use shared stash.

## Ownership and interface

- Role: frontend_implementer. Validator: frontend_validator. Reviewer: reviewer (independent, root dispatched).
- Owned files: `apps/mobile/src/delivery-integration-smoke.ts`, `apps/mobile/src/delivery-integration-smoke.test.ts` only.
- Consumes: existing `createDeliveryRecovery`, `createMobileCodeDeliveryRemote`, `deliveryIntegrationConfig`, authorized runtime request and scope lease, as specified in plan Task 8.
- Produces: optional env flag `WEKNORA_MOBILE_TEST_DELIVERY_RECOVER=1`; evidence fields `recovery: skipped|not-needed|recovered|failed` and optional `recoveryState`; opt-in recovery run after successful delivery read. `DELIVERY_STATE_CONFLICT` / `DELIVERY_INVALID_INPUT` mean `not-needed`; other errors mean `failed` and append failure evidence. No behavior or network access when opt-in is off.

## Required steps

1. Read the cited fact sources and inspect current files and Task 5/6 seams.
2. RED: add/adjust tests for opt-in default off, enabled config, and recovery outcome mapping where current test seams allow deterministic coverage. Record exact failing command before implementation.
3. GREEN: implement only the owned files with the existing runtime and remote; ensure recovery uses the exact read delivery ID and run ID. Failures must be represented honestly; no retries or fabricated evidence.
4. REFACTOR: keep evidence types narrow and behavior fail-closed.
5. Run `pnpm exec tsx --test apps/mobile/src/delivery-integration-smoke.test.ts && pnpm --filter @weknora/mobile typecheck`; run `git diff --check`; report exact output/status. Do not claim live deployment proof without actual opt-in credentials/environment. Do not inspect secret values.
6. Commit implementation and tests; create `.superpowers/sdd/plan-t55/task-8-report.md`, exact BASE-to-implementation patch and SHA-256. Report RED/GREEN, files, commands, outcomes, limitations, commits.

## Acceptance and review focus

- With opt-in absent or off, recovery remains `skipped` and the existing integration flow is unchanged.
- With opt-in on after delivery read, recovery records `recovered` plus state, `not-needed` for documented state/input conflicts, or `failed` and a failure detail for all other errors.
- No delivery read means no recovery attempt.
- Spec and quality review must assess honest evidence, no accidental retries, correct runtime scope/lease, error classification, type safety, and test coverage.

## Status

Task 8 is ready after Task 6 verified. It is independent of T63 Task 3. No downstream claim of #55 completion until task review and integration succeed.
