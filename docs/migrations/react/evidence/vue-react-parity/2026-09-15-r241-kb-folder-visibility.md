# R241 knowledge-base folder visibility

## Runtime finding

On the same authenticated empty KB (`Parity KB Demo`), Vue's `showFolderTree` is `hasFolders`, where the synthetic Root row does not count as a real folder. Vue therefore hides the folder column and lets the document filter span the content width. React previously rendered its flattened Root row as an always-visible `wk-folder-panel`, moving the results/filter surface to `x=536`.

## Change

`KnowledgeDocumentsPage.tsx` now derives `showFolderTree` from real non-empty folder paths. The folder aside and two-column grid are rendered only when a real folder exists; otherwise the existing project controls and document surface use a single full-width column. Filtering, upload, permissions and folder navigation contracts remain unchanged.

## Verification

- Vue source baseline: `frontend/src/views/knowledge/KnowledgeBase.vue:639-643` (`hasFolders` and `showFolderTree`).
- Focused document chrome tests: `pnpm exec tsx --test apps/web/src/documents/page-chrome.test.tsx` — 9/9 passed.
- Web typecheck: `pnpm run typecheck:web` — passed.
- Full Web regression after the conditional folder change: `pnpm run test:web` — 899/899 passed.
- Browser runtime at 1355x720, DPR2, zh-CN: React empty KB measured `folder=null`, layout/results/filter `x=296,y=139.59,w=1031`; Vue showed the same no-folder surface.

Real-folder two-column rendering, non-empty document cards, upload/preview mutations, responsive and desktop/native evidence remain open.
