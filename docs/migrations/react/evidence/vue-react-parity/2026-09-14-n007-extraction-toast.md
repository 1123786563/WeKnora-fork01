# N007 — graph extraction feedback toast

Date: 2026-09-14

## Vue baseline

`frontend/src/views/knowledge/settings/GraphSettings.vue` reports graph tag/text/relation extraction success and failure through TDesign `MessagePlugin` toasts. The feedback is outside the upload-confirm section and does not add an inline error block to the dialog layout.

## React implementation

`apps/web/src/documents/KnowledgeDocumentsPage.tsx` now routes graph extraction feedback, file-add feedback, and URL duplicate/add feedback through a page-level transient toast. The toast is top-centered, announced with `role="alert"`/`aria-live="polite"`, auto-dismisses after 3 seconds, and cleans up its timer on unmount. Upload pipeline failures remain inline because Vue keeps those per-file/per-confirmation errors visible for correction.

## Verification

- Focused upload-confirm and pipeline suites: 50/50 passed.
- Full Web suite: `pnpm run test:web` — 681/681 passed.
- Web typecheck: `pnpm run typecheck:web` — passed.
- `git diff --check` — passed.
- Browser extraction failure against a live graph-enabled backend remains unavailable; the current deployment has graph extraction disabled, so runtime endpoint acceptance is `blocked-env`.

Status: implementation and static/component regression verified; live graph extraction and cross-app screenshot evidence remain open.
