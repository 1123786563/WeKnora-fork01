# React/Vue parity evidence — chat model option sanitization

Date: 2026-09-15

## Change

Chat model options now trim identifiers and labels and omit malformed rows with an empty id or label. The display-only model chip also falls back to the localized unconfigured label when the selected model has no usable name, preventing blank controls when the backend returns incomplete model metadata.

## Validation

- `pnpm run test:web` — 895/895 passed.
- `pnpm run typecheck:web` — passed.
- `pnpm run build:web` — passed.
- `git diff --check` — passed.

## Runtime observation

The authenticated chat tenant currently returns an empty model option, so the selector is intentionally hidden after sanitization and the chip remains localized. A tenant with valid multiple KnowledgeQA model records is still required for live multi-model selection evidence.
