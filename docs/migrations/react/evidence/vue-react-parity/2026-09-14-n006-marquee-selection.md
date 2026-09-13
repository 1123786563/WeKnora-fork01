# N006 document marquee selection parity

Date: 2026-09-14

## Vue baseline

`frontend/src/hooks/useMarqueeSelect.ts` defines a left-button drag gesture with a 6px threshold. The gesture ignores buttons, links, inputs, selects and labels. Its mode is fixed from the start point: an unselected row or blank space adds intersecting rows, while a selected row subtracts intersecting rows. The selection rectangle is positioned in the scroll container and rows are selected when their client rectangles intersect it.

## React implementation

`apps/web/src/documents/selection.ts` now provides `useMarqueeSelection` and the pure `applyMarqueeSelection` helper. `KnowledgeDocumentsPage.tsx` attaches the gesture to the document list, marks rows with `data-select-id`, renders an add/subtract rectangle, and leaves row controls available because control targets are excluded from starting a gesture. Existing checkbox, Shift-range, filter-reset and page-select-all behavior remains intact.

## Verification

- Focused selection tests: 5/5 passed, including add/subtract marquee semantics and existing Shift-range semantics.
- Focused tag tests: 11/11 passed, including the responsive Vue overflow-limit calculation.
- Web test suite: 763/763 passed.
- `pnpm typecheck:web`: passed before the subsequent chip-only change; the later rerun is blocked by parallel FAQ test files missing exports from `FAQPage`.
- `pnpm build:web`: passed (2341 modules transformed; existing large-chunk warning only).

## Evidence boundary

This records source-level and automated behavior evidence. Authenticated same-condition Vue/React browser drag screenshots, real backend batch-operation verification, and Wails/iOS/Android evidence remain open; this slice is not marked final acceptance.
