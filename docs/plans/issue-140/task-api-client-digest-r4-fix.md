# Task Report: API client preparation snapshot digest (OCR round 4)

## Brief

Fix the confirmed valid medium finding in `docs/plans/issue-140/ocr/round4-highrisk-analysis.md`: for non-draft preparation receipts, a malformed `snapshotSha256` must be rejected whenever any snapshot identity field is supplied, including `snapshotSha256` itself, even when `opportunityId` and `snapshotId` are absent.

## Changes

- `packages/api-client/src/career.ts`: non-draft snapshot validation now checks that any provided snapshot identity value is accompanied by a valid SHA-256 digest.
- `packages/api-client/src/career.test.ts`: added a regression payload with only malformed `snapshotSha256` and confirmed the decoder rejects it.

## Verification

- RED: `pnpm exec tsx --test packages/api-client/src/career.test.ts` failed before the implementation; the new payload caused “Missing expected rejection (TypeError)”.
- GREEN: `pnpm exec tsx --test packages/api-client/src/career.test.ts` — 51 passed, 0 failed.
- `pnpm typecheck:web` — passed (TypeScript completed with exit code 0).
- `git diff --check -- packages/api-client/src/career.ts packages/api-client/src/career.test.ts docs/plans/issue-140/task-api-client-digest-r4-fix.md` — passed; reviewed the tracked source and test diff. The report is newly created and contains only this task’s evidence.

## Scope and risks

Only the non-draft snapshot digest requirement changed. Existing draft requirements remain intact. A non-draft receipt with no snapshot fields remains valid; any provided opportunity ID, snapshot ID, or digest now requires a well-formed digest.
