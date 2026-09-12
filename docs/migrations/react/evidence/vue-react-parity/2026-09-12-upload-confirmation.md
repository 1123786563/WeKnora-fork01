# Upload confirmation parity evidence

Date: 2026-09-12

## Scope

This slice covers the React file staging and confirmation surface corresponding to the Vue `UploadConfirmHost.vue` and the file-mode portion of `UploadConfirmDialog.vue`: drag/drop staging, multi-file summary, per-file sequential status, shared tags, cancellation, retry after per-file failure, and removal of an individual staged file.

It remains `implementing`. It does not claim parity for the Vue parser/chunking settings, PDF/multimodal/ASR configuration, mixed URL and file batches, destination-folder picker, manual/reparse modes, six-locale copy, fixed-viewport screenshots, or real-backend/native acceptance.

## Evidence

| Layer | Command / result | Classification |
|---|---|---|
| Focused pipeline | `pnpm exec tsx --test apps/web/src/documents/upload-pipeline.test.ts` — 6 passed, 0 failed | pure business regression evidence |
| Static | `git diff --check` — 0 | static evidence |
| Browser / real backend | not run in this slice | missing evidence |
| Wails / iOS / Android | not run in this slice | missing evidence |

## Remaining work

The confirm shell must be expanded from the current file-only subset to the Vue state model before review: URL items and add-more behavior, target folder, parse configuration, manual/reparse source previews, locale-derived feedback, and actual browser interaction evidence.
