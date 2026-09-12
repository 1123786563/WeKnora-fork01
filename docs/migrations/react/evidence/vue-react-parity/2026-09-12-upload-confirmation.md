# Upload confirmation parity evidence

Date: 2026-09-12

## Scope

This slice covers the React file/URL staging and confirmation surface corresponding to the Vue `UploadConfirmHost.vue` and the file-mode portion of `UploadConfirmDialog.vue`: drag/drop staging, multi-file summary, per-file sequential status, shared tags, cancellation, retry after per-file failure, removal of an individual staged file, and URL confirmation before the backend mutation.

It remains `implementing`. It does not claim parity for PDF configuration, mixed URL and file batches, destination-folder picker, manual/reparse modes, six-locale copy, fixed-viewport screenshots, or real-backend/native acceptance. The basic Vue chunking fields, server-backed parser-engine rules, multimodal VLM settings, and ASR settings now initialize from the KB when present, validate required model IDs when enabled, and are sent in the backend-supported `process_config` for file, URL, and manual creation; when a folder is selected in the existing document tree, uploaded files/URLs are moved through the existing folder API after creation. A dedicated destination picker is still missing.

## Evidence

| Layer | Command / result | Classification |
|---|---|---|
| Focused pipeline | `pnpm exec tsx --test apps/web/src/documents/upload-pipeline.test.ts` — 7 passed, 0 failed | pure business regression evidence |
| Web full regression after multimodal/ASR wiring | `pnpm test:web` — 282 passed, 0 failed | Web regression evidence |
| Parser engine contract/type integration | `client.knowledgeBases.settings.parserEngines()` loads the server registry; selected `parser_engine_rules` are included in file, URL, and manual `process_config` | shared-contract integration evidence |
| Web typecheck | `pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit` — passed | Web static integration evidence |
| Web test suite | `pnpm test:web` — 279 passed, 0 failed | Web regression evidence |
| Static | `git diff --check` — 0 | static evidence |
| Browser / real backend | not run in this slice | missing evidence |
| Wails / iOS / Android | not run in this slice | missing evidence |

## Remaining work

The confirm shell must still be expanded to the Vue state model before review: mixed URL/file items and add-more behavior, dedicated target-folder picker, PDF configuration, provider-backed model selectors for multimodal and ASR, manual/reparse source previews, locale-derived feedback, and actual authenticated browser interaction evidence.
