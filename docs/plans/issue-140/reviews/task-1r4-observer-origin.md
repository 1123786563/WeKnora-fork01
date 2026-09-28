# Issue 140 Task 1R4 observer origin evidence

## Status

Task 1R4 implementation complete; Task 1 remains blocked pending independent review.

## Review package

- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t01-contracts/WeKnora-fork01`
- Branch: `codex/issue-140-t01-contracts`
- BASE: `4430361efc34a87a084ca3d65650f1d31c83bf2a`
- Implementation HEAD: `b48c24f9fef07c52de6c4f6eea3fc3602a983a72`
- Implementation commit: `b48c24f9f fix: scope career observer to client deployment`
- Evidence commit: separate from implementation.
- Patch: `docs/plans/issue-140/reviews/task-1r4-observer-origin.patch.gz`
- Patch SHA-256: `0ee2efd998e2c6367467e1f4d802d263bb5aa0ee4aa6f7fc2ddbf4b66ec4be88`

## Change

The Career HTTP requester and revision observer adapter now call the same deployment-origin guard. A configured provider is invoked only after the observer scope matches the authenticated client's `baseURL` origin. Tenant and actor authorization remain the provider's responsibility. The test compares the observer mismatch error to the existing HTTP mismatch error and asserts provider invocation count remains zero. The existing matching-origin hint → decoded refresh → Desk projection → dispose/unsubscribe test remains the positive control.

## TDD evidence

- RED: `pnpm exec tsx --test --test-name-pattern="outside the authenticated client scope" packages/api-client/src/client.test.ts`: failed because observer registration did not throw.
- GREEN: `pnpm exec tsx --test --test-name-pattern="outside the authenticated client scope|configured production observer hints|ordinary production client fails observation" packages/api-client/src/client.test.ts`: pass, 3/3.

## Verification

- `pnpm exec tsx --test packages/contracts/src/career/*.test.ts packages/api-client/src/career/*.test.ts packages/career-core/test/*.test.ts packages/api-client/src/client.test.ts`: pass, 41/41.
- `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2022 --lib ES2022,DOM --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions packages/contracts/src/index.ts packages/api-client/src/client.ts packages/career-core/src/index.ts`: pass.
- `pnpm test:shared`: pass, 1,146 passed, 4 skipped (1,150 total).
- `pnpm typecheck:shared`: only known errors remain in unchanged `packages/views/src/chat/mermaid.ts:127,158` (theme `darkMode` literal mismatch and unsupported `themeVariables`).
- `git diff --check 4430361efc34a87a084ca3d65650f1d31c83bf2a..b48c24f9fef07c52de6c4f6eea3fc3602a983a72`: pass.

## Scope

Implementation changes are limited to `packages/api-client/src/client.ts` and `packages/api-client/src/client.test.ts`. No Task 2 work is released; Task 1 stays pending independent review.
