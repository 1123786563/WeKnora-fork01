# N011 Wiki reader/editor mode evidence

Date: 2026-09-14

## Vue baseline

`frontend/src/views/knowledge/wiki/WikiBrowser.vue` keeps a selected page in a read-only reader by default. The editor is entered through an explicit edit action or the new-page action. Successful save, reload, and revert return to the selected-page state.

## React implementation

`apps/web/src/wiki/WikiPage.tsx` now models the same transition: selecting a page clears edit mode and renders `.wk-wiki-reader`; `Edit` enters the existing editor; `New page` opens create mode; successful save/reload/revert selects the returned page and leaves edit mode. Reader content uses a scoped `.wk-wiki-reader-content` style.

## Verification

- Web test suite: 793/793 passed.
- Web TypeScript check: passed with `--incremental false`.
- Evidence class: static/unit and typecheck only.
- Not claimed: authenticated browser comparison, real-backend state matrix, Wails desktop, iOS, or Android runtime evidence.

## Remaining N011 gaps

Index/tree/list modes, folder actions, graph state, computed-style screenshot comparison, and platform/runtime evidence remain open.
