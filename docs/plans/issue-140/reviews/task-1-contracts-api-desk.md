# Task 1 implementation evidence and review package

## Scope and checkpoint

- Task: `docs/superpowers/plans/2026-09-28-issue-140-main-port.md`, Task 1.
- Brief: `.superpowers/sdd/2026-09-28-issue-140-main-port/task-1-brief.md`.
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t01-contracts/WeKnora-fork01`.
- Branch: `codex/issue-140-t01-contracts`.
- BASE: `38d2d1d037ab9b15470f4f7f15bc84de3f138d83`.
- Implementation HEAD: `75b6343583fb91f15c165877d7d3f6fa24935fc6` (`feat: add versioned career contracts and shared desk`).
- Full implementation review range: `38d2d1d037ab9b15470f4f7f15bc84de3f138d83..75b6343583fb91f15c165877d7d3f6fa24935fc6`.
- `git diff --check BASE..HEAD`: PASS. Implementation change: 23 files, 289 insertions, 1 deletion.

## Delivered

- Added strict Career DTOs and parsers for confirmed profile facts, immutable opportunity snapshots, tri-state evaluation, application stage/snapshot references, material versions/digests and privacy receipts. Unknown fields and tenant/owner/actor authority fields are rejected at client decoding boundaries.
- Added `CareerApi` and `createCareerApi` with versioned `/api/v1/career/...` paths, expected revision and request ID headers for writes, and strict response decoding. Exposed it through the shared API client and its package exports.
- Added platform-neutral `@weknora/career-core` Desk seam for open/list/act/observe, persistent request intent before writes, same-ID outcome reconciliation, explicit revision rebase, and scope-switch invalidation of pending intents and late reads/writes.
- Added workspace/package exports and test wiring. `pnpm install --offline` updated the lockfile for the new workspace package.

## TDD and verification evidence

- RED: `pnpm exec tsx --test packages/contracts/src/career/index.test.ts packages/api-client/src/career/index.test.ts packages/career-core/test/desk.test.ts` failed because the Career contract, API decoder and Desk entry modules did not exist (`ERR_MODULE_NOT_FOUND`).
- GREEN targeted: same command after implementation — PASS, 4 tests, 0 failures.
- Package suites: `pnpm --filter @weknora/contracts test` — PASS, 1 test; `pnpm --filter @weknora/api-client test` — PASS, 1 test; `pnpm --filter @weknora/career-core test` — PASS, 2 tests.
- Focused strict TypeScript check: `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2022 --lib ES2022,DOM --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions packages/contracts/src/index.ts packages/api-client/src/client.ts packages/career-core/src/index.ts` — PASS.
- Shared regression: `pnpm test:shared` — PASS, 1130 tests, 1126 passed, 4 skipped, 0 failed.
- `pnpm typecheck:shared` — FAILS only at pre-existing unrelated `packages/views/src/chat/mermaid.ts:127` (`darkMode: true` incompatible with inferred `false`) and `:158` (`themeVariables` not in config type). No changes were made to this unrelated file. The same failure was recorded in the assigned baseline.
- Dependency installation: `pnpm install --offline` — PASS; workspace resolved, no downloads. Existing peer/deprecation/build-script warnings remain.

## Source and interface notes

- Read the approved port design, product spec, `CONTEXT.md`, ADRs 0015–0018 and Task 0 final verification report.
- ADR 0019 is referenced by the approved port design and Task 0 verification report but its file is absent from this worktree (`rg --files docs/adr` lists no 0019). The Desk interface follows the explicit Task 1/Task 0 statements: contracts own decoded DTOs, API client owns HTTP, Desk owns cross-client intent/revision/scope recovery; clients supply platform persistence/adapters.
- The checked-in Task 1 Brief predates Task 0's verified plan correction: it says not to add `career-core`, while the current implementation plan and Task 0 final verification include the package in Task 1. The controller's assigned task message explicitly included the new package, so this checkpoint follows that current assignment.
- Existing `ClientRequest` in `packages/api-client/src/client.ts` is the per-call HTTP request object. Therefore `createCareerApi` accepts the repository's request callback (`CareerRequester`) that consumes `ClientRequest` values, which matches the existing client construction pattern and preserves the existing `ClientRequest` type.

## Review handoff

Review the full BASE..implementation HEAD range above. Task 1 is not marked verified here; independent review and validation remain controller responsibilities.
