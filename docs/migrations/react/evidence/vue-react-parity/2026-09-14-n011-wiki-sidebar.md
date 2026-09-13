# N011 Wiki browser sidebar parity

Date: 2026-09-14

## Vue baseline

`frontend/src/views/knowledge/wiki/WikiBrowser.vue` renders a fixed-width left browser with an inset search field, scrollable page rows, summary clamped to two lines, compact metadata, hover/selected container state, and a centered empty state. The reader/editor owns the right column.

## React change

`apps/web/src/wiki/WikiPage.tsx` now uses `wk-wiki-sidebar` and `wk-wiki-page-list` for the left browser. Page rows expose title, summary, version and selected state; the search field is inside the sidebar; the generic bordered `wk-toolbar` wrapper is removed. `apps/web/src/styles.css` adds scoped Vue-derived dimensions/colors and preserves the existing editor, save-conflict, history and diff behavior.

## Evidence

| Layer | Result |
| --- | --- |
| Focused structural test | `WikiPage.test.tsx` + `editor.test.ts`: 4/4 pass |
| Full Web suite | 657/664 in the current concurrent tree; 7 failures are in other in-flight documents/tags tests, not Wiki |
| Typecheck/build | Not rerun after this slice |
| Browser computed styles | Not freshly captured; paired Vue/React browser harness requires the authenticated Vue session |
| Real backend / Wails / iOS / Android | Not claimed for this slice |

Status remains `review`: the Vue browser also contains index/tree/list modes, folder actions, graph view and reader states that need their own runtime evidence before N011 can be accepted.
