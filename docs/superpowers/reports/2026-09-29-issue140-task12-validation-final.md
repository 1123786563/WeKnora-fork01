# Issue 140 Task 12 N9 Validation (Final)

Date: 2026-09-29

## Revision and scope

- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-sweep-integration/WeKnora-fork01`
- Validated revision: `3712062df7d672a77d7411aa055660d13d63351a` (`test(miniprogram): verify deletion retries stay scope local`)
- Validated file: `apps/miniprogram/tests/export-deletion.test.mjs`
- Validated file SHA-256: `be820c3f00a83fa927fd02cf9cfaaeb5e9a9ce44316a1f09013c586396805d5e`
- The tests were run while this same content was the working change based on `e0fa1597d33fbfc03f62b646942447ddaaae8b09`; that change was committed during validation. Its diff SHA-256 was `1f820c6cf25387b65bbcd446e5843a3ec2021dea40112f05052944ca9886782e`. The committed test file hash above confirms the validated bytes.
- Runtime: Node `v24.18.1`, pnpm `10.28.2`

## Commands and results

All checks below completed against the file content identified above. After the validation commands completed, the source change was committed at the validated revision; no further source changes were observed.

1. From `apps/miniprogram`:
   ```sh
   /Users/wuyongjun/.nvm/versions/node/v24.18.1/bin/node --experimental-transform-types --test --test-name-pattern='N9:' tests/export-deletion.test.mjs
   ```
   Exit 0; 3 tests, 3 passed, 0 failed, 0 skipped.

2. From `apps/miniprogram`:
   ```sh
   /Users/wuyongjun/.nvm/versions/node/v24.18.1/bin/node --experimental-transform-types --test tests/export-deletion.test.mjs
   ```
   Exit 0; 33 tests, 33 passed, 0 failed, 0 skipped.

3. From repository root:
   ```sh
   PATH=/Users/wuyongjun/.nvm/versions/node/v24.18.1/bin:$PATH pnpm --filter @weknora/miniprogram test
   ```
   Exit 0; 215 tests, 214 passed, 0 failed, 1 skipped (opt-in real-deployment integration test).
   pnpm printed the package engine warning (`wanted >=26`, current `v24.18.1`); the requested runtime was used and the suite passed. Node also printed its expected experimental type stripping warning.

4. From repository root:
   ```sh
   git diff --check
   ```
   Exit 0; no whitespace errors.

## Acceptance result and risks

The assigned N9 validation passes. The success retry test verifies that a pre-existing intent belonging to another scope survives completion, and the ambiguous retry test verifies that both scopes' intents remain intact after a scope switch. No acceptance gaps were observed in the assigned test scope.

The sole validation caveat is the package's declared Node `>=26` engine versus the explicitly requested Node `24.18.1`; despite pnpm's warning, the full package test command exited successfully. The opt-in real-deployment test was skipped because deployment credentials/configuration were not provided.
