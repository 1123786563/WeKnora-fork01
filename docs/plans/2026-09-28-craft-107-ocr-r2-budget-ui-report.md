# Craft 107 OCR R2 — Budget UI Report

Checkpoint: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159` (unchanged HEAD; no commit).

## Scope and result

- F12: extension button callback now follows `budgetPauseView.canExtend`, the server's owner/billing-admin projection, rather than the owner-only `canWrite` gate.
- F13: one generated idempotency key is retained per run until extension succeeds. A ref blocks synchronous double submission, and the button is disabled while the request is in flight. Rejected/ambiguous requests retain their key for retry.
- F14 is resolved by Task H's server-owned `data.extension_action` intent and quantum. The backend contract and evidence are in `/Users/wuyongjun/.codex/worktrees/ocr-budget-auth/WeKnora-fork01/docs/plans/2026-09-28-craft-107-ocr-r2-budget-intent-backend-report.md`.

## TDD and verification evidence

- RED: with the old `onRequestExtension={canWrite ? ...}` gate restored temporarily, `node --import tsx --test --test-name-pattern='server-authorized collaborator' src/features/craft/routes.access.test.tsx` failed at `routes.access.test.tsx:200`: expected the authorized collaborator button to be enabled, actual `disabled === true`.
- GREEN: after the fix, `node --import tsx --test src/features/craft/routes.access.test.tsx` passed all 11 tests, 0 failed. The new test covers the non-owner authorized action, disabled in-flight state, transient failure retry, and same idempotency key replay.
- `git diff --check` passed.
- Initial package-script attempt could not resolve `tsx` because worktree dependencies were absent. `pnpm install --offline --frozen-lockfile` populated dependencies without changing the lockfile; the direct targeted test command above then ran successfully.
- Interaction evidence: the button becomes disabled while pending, becomes enabled after transient failure, and sends the same key on retry. Existing Craft locale strings continue to provide the button text; native button disabled semantics remain intact. No layout changes were made.

## Changed files and checkpoint hashes

- `apps/web/src/features/craft/routes.tsx` — `1978d9c915c23ec5989dae77c94d5fac60709117c7ef3f091c7387c892c5c1d3`
- `apps/web/src/features/craft/routes.access.test.tsx` — `ce96bf00073eb07bb6dfd536da3fb4a0fc8eb0b733a856e9114a04842a1cbf20`
- `packages/api-client/src/craft/index.ts` — `b3bf9d2a575f9a5aefc0fc52d24de76f3808a1dd9ca35cb52eff6e2d5c39819a`
- `packages/api-client/src/craft/index.test.ts` — `1e5b7b301aa64e1ed0ed9d56a86ae7e2799728316f4b84371cafb706e2359d56`
- `packages/api-client/src/index.ts` — `0d26abd0f0b6d4c1fbe6a946353aebe460935c6cf5e0c22d559ef16aeccb0fee`

`HEAD` remains the assigned base and no commit was created.

## F13 Review Fix — Round 1/5

- Persisted each pending key in `localStorage` under a key scoped by encoded `sessionId` and `runId`. The in-memory cache uses the same compound scope.
- The key stays pending after ambiguous request failure and survives a full `CraftRoutes` unmount/remount. A confirmed extension response clears it. If the authoritative workspace later shows that the Run left `budget_exhausted`, that projection also retires a stale key after a lost response. A later pause for the same run therefore receives a fresh key.
- Storage reads/writes fail closed before sending the request. A cleanup failure after confirmed success is surfaced and tells the user to reload before asking again. Both storage errors render beside the budget notice in Chinese or English according to the selected Craft locale.
- RED: `node --import tsx --test --test-name-pattern='ambiguous extension retry' src/features/craft/routes.access.test.tsx` failed before the fix because no unresolved key existed in browser storage after the ambiguous response.
- GREEN: `node --import tsx --test src/features/craft/routes.access.test.tsx` passed all 13 tests, 0 failed. New coverage verifies same-key replay after unmount/remount, a fresh key for a later pause after confirmed success, and fail-closed behavior when browser storage is unavailable.
- `git diff --check` passed. HEAD is still `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no commit was created.

## F13 Review Fix — Round 2/5

- Every extension POST now rereads the scoped `localStorage` entry, including retries that have a cached key in the mounted component. The POST is allowed only if storage still contains the exact cached key; a missing/mismatched entry or storage read failure blocks the retry before the request is sent.
- Added localized, budget-panel errors for storage verification failure and missing/changed pending keys.
- RED: `node --import tsx --test --test-name-pattern='persisted key is missing or unavailable' src/features/craft/routes.access.test.tsx` failed before the fix: removing the stored entry still produced a second `/budget/extend` POST (`2 !== 1`).
- GREEN: `node --import tsx --test src/features/craft/routes.access.test.tsx` passed all 14 tests, 0 failed. The new regression covers both removing the key and making storage unavailable after an ambiguous first request. Existing coverage continues to verify remount replay and a fresh key after confirmed success.
- `git diff --check` passed. HEAD remains `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no commit was created.

## Task H Frontend Integration

- Replaced component/localStorage key generation with the server's durable `extension_action`. GET parsing returns its exact `{key, extra_calls, extra_credits}` tuple; extension POST accepts and forwards that tuple unchanged. The API types are exported through the package entry point.
- The pause panel shows an extension action only when the server returned a non-null action. `can_extend` alone cannot create an action. The route uses the server action as the only authority and quantum source.
- A retry in the same component reuses the projected action. After a component remount, the route fetches the pause view again and uses the backend's same pending action. A later pause uses the backend's newly issued action. The in-flight guard continues to disable repeat submissions.
- This server-owned intent supersedes the R1/R2 browser-storage workaround and its storage failure tests; no `localStorage` reference remains in `routes.tsx`.
- RED: API test `node --import tsx --test --test-name-pattern='budgetPause reads the server extension action' src/craft/index.test.ts` first failed because `extensionAction` was `undefined`. Route collaborator test first failed because the POST used a newly generated key and hardcoded 10 / 10,000,000 instead of the server tuple.
- GREEN: `node --import tsx --test src/features/craft/routes.access.test.tsx` passed 13/13; `node --import tsx --test src/craft/index.test.ts` from `packages/api-client` passed 12/12.
- Typecheck: Node `v26.7.0`, `./node_modules/.bin/tsc --noEmit -p apps/web/tsconfig.json` passed with exit 0 and no diagnostics.
- `git diff --check` passed. `HEAD` remains the assigned base and no commit was created.
