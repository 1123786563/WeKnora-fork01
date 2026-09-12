# Upload confirmation parity evidence

Date: 2026-09-12

## Scope

This slice covers the React file/URL staging and confirmation surface corresponding to the Vue `UploadConfirmHost.vue` and the file-mode portion of `UploadConfirmDialog.vue`: drag/drop staging, multi-file summary, per-file sequential status, shared tags, cancellation, retry after per-file failure, removal of an individual staged file, and URL confirmation before the backend mutation.

It remains `implementing`. It does not claim parity for the remaining PDF workflow details, full folder-picker behavior, manual/reparse modes, six-locale copy, fixed-viewport screenshots, or real-backend/native acceptance. The basic Vue chunking fields, server-backed parser-engine rules, PDF scanned-parser override, multimodal VLM settings, and ASR settings now initialize from the KB when present, use existing tenant model inventory selectors when available, validate required model IDs when enabled, and are sent in the backend-supported `process_config` for file, URL, and manual creation; the PDF override is only emitted for a staged PDF batch. A staged URL can coexist with files and is processed before the file pipeline; failed URL/file work remains staged for retry. The confirmation surface now lets the user choose root, an existing folder, or a validated new folder path, defaulting to the folder being browsed, and moves created URL/files through the existing folder API.

## Evidence

| Layer | Command / result | Classification |
|---|---|---|
| Focused pipeline | `pnpm exec tsx --test apps/web/src/documents/upload-pipeline.test.ts` — 7 passed, 0 failed | pure business regression evidence |
| Web full regression after multimodal/ASR wiring | `pnpm test:web` — 282 passed, 0 failed | Web regression evidence |
| Parser engine contract/type integration | `client.knowledgeBases.settings.parserEngines()` loads the server registry; selected `parser_engine_rules` are included in file, URL, and manual `process_config` | shared-contract integration evidence |
| Provider model inventory integration | `client.configuration.models.list()` supplies VLLM/ASR selector options with manual-ID fallback | shared-contract integration evidence |
| PDF override integration | PDF batches expose the Vue `pdf_force_scanned` option and submit it only when a PDF is staged | process-config integration evidence |
| Mixed batch staging | File selection appends to existing staged files; a staged URL is retained and processed before files, with failed work preserved | state/mutation integration evidence |
| Destination selection | Confirmation surface offers root and loaded folder paths; selected non-root destination is applied after URL/file creation | folder-routing integration evidence |
| New destination path | New folder path input rejects empty/traversal segments and applies the path through the existing move-to-folder contract | folder-routing validation evidence |
| Web typecheck | `pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit` — passed | Web static integration evidence |
| Web test suite | `pnpm test:web` — 279 passed, 0 failed | Web regression evidence |
| Static | `git diff --check` — 0 | static evidence |
| Browser / real backend | not run in this slice | missing evidence |
| Wails / iOS / Android | not run in this slice | missing evidence |

## Remaining work

The confirm shell must still be expanded to the Vue state model before review: picker visual/navigation parity, full PDF section presentation, manual/reparse source previews, locale-derived feedback, and actual authenticated browser interaction evidence.
