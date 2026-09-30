# Issue 140 Task 1R3 observation contract evidence

## Status

Implementation complete for Task 1R3; Task 1 remains **unverified**, and downstream work remains blocked pending independent review.

## Review package

- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t01-contracts/WeKnora-fork01`
- Branch: `codex/issue-140-t01-contracts`
- BASE: `7e209f5867f63c4b559e20e4b76e1773808252b3`
- Implementation HEAD: `e2040f7703c8234724fc9ad78f0080d3c8430f64`
- Implementation commit: `e2040f770 fix: make career observation support explicit`
- Evidence commit is separate from implementation commit.
- Patch: `docs/plans/issue-140/reviews/task-1r3-observation-contract.patch.gz`
- Patch SHA-256: `fac2713437bac13d93cfced082949f3661cc6a4daa9b9ca4b004e2837be4fbe1`

## Change summary

- `createCareerApi` no longer creates an inert default observer. Calling `observe()` without a configured provider throws exported `CareerObservationUnavailableError` immediately.
- Main API exports include the typed error and `CareerObserver` type. `WeKnoraClientOptions.careerObserver` documents the runtime-provided hook.
- ADR 0019 names the provider contract `(scope, onRevision) => unsubscribe`: provider hints only trigger a CareerApi `open()`, whose strict decoder gates Desk publication. Since `HttpTransport` currently exposes no Career event source, consumers handle unsupported observation by explicitly refreshing with `open()` or disabling observation-dependent freshness behavior.
- Assembled tests cover the no-provider error, configured provider hint → CareerApi request/decode → Desk projection update, and unsubscribe via Desk disposal.

## TDD evidence

- RED: `pnpm exec tsx --test --test-name-pattern="ordinary production client fails observation|configured production observer hints" packages/api-client/src/client.test.ts` failed as expected because no exception was thrown for the unconfigured observer. The configured-provider case already passed with the earlier injected provider; the missing-source regression was the failing case.
- GREEN: `pnpm exec tsx --test --test-name-pattern="ordinary production client fails observation|configured production observer hints|production Career client forwards" packages/api-client/src/client.test.ts`: **pass**, 3/3.

## Verification

- `pnpm exec tsx --test packages/contracts/src/career/*.test.ts packages/api-client/src/career/*.test.ts packages/career-core/test/*.test.ts packages/api-client/src/client.test.ts`: **pass**, 40/40.
- `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2022 --lib ES2022,DOM --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions packages/contracts/src/index.ts packages/api-client/src/client.ts packages/career-core/src/index.ts`: **pass**.
- `pnpm test:shared`: **pass**, 1,145 passed, 4 skipped (1,149 total).
- `pnpm typecheck:shared`: **fails only at unchanged baseline** `packages/views/src/chat/mermaid.ts:127,158` (theme literal mismatch and unsupported `themeVariables`). No unrelated failures were modified.
- `git diff --check 7e209f5867f63c4b559e20e4b76e1773808252b3..e2040f7703c8234724fc9ad78f0080d3c8430f64`: **pass**.

## Scope and limitation

Changes are limited to the Career API/client observer contract and tests, ADR 0019, and this evidence/plan record. This package does not add a server event endpoint or transport polling. Each runtime that needs revision observation must provide `careerObserver`; other consumers must explicitly refresh with `open()` or avoid depending on pushed freshness. Independent review remains required before Task 1 can be marked verified.
