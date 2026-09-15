# Documents parity agent report

Date: 2026-09-15

## Scope

Implemented only the React documents module under `apps/web/src/documents` plus its dedicated tests. Existing unrelated worktree changes were preserved. The requested module is exported from `apps/web/src/documents/index.ts`; route/App wiring was intentionally not changed because the task scope forbids edits outside `documents` (the current React shell has no documents route yet).

## Vue alignment covered

- List loading, pagination, keyword/status filtering, empty/error/retry and forbidden/read-only branches.
- Upload selection validation, multi-file upload, upload tags, submitting deduplication, failure feedback and refresh on success.
- Document detail dialog, preview action, metadata display and processing timeline.
- Vue status precedence: pending/processing, finalizing, summary processing, failed, cancelled, draft and completed.
- Row selection, select-all, clear selection, batch reparse/delete with confirmation and disabled submitting state.
- Permission gating for preview/download versus edit/reparse/cancel/delete/tag mutations.
- Explicit `DocumentsApi` seam for list/detail/preview/upload/tag/reparse/cancel/delete/download so backend contracts are not invented or duplicated in shared packages.

## Evidence

- RED: `model.test.ts` initially failed with `ERR_MODULE_NOT_FOUND` for the intentionally absent `model.ts`.
- GREEN: `pnpm --filter @weknora/web exec node --import tsx --test src/documents/model.test.ts src/documents/list.test.ts` — 7 passed, 0 failed.
- `git diff --check` completed without whitespace errors.
- `pnpm --filter @weknora/web build` — passed (`tsc -b` and Vite production build).
- `git diff --check` — passed.
- No live Vue/React browser or backend acceptance was claimed: the current task did not authorize route wiring or modify the shared client to expose the mutation endpoints.
