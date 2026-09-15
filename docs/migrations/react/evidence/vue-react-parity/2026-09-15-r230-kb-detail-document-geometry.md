# R230 knowledge-base document detail geometry

## Baseline

Vue and React were opened in Chrome with the same authenticated user, KB (`Parity KB Demo`), zh-CN locale, viewport 1355x720 and DPR2. Vue's `/platform/knowledge-bases/:kbId` detail page is the authority for the disabled-Wiki fixture.

Before this change, React added shadcn Card chrome around the document area, used a 48px page inset, and allowed the shared search input's content-box padding to expand its outer height to 50px. Vue has no extra Card border/padding, uses a 24px top inset with left/right content insets of 36px/28px, and uses 32px controls.

## Change

`KnowledgeDocumentsPage.tsx` now uses a borderless document surface, Vue-derived page insets, a 32px title line box, a 32px border-box document search input, and a 32px filter row. The page still uses the project Input/Select/Checkbox/Button wrappers and keeps all upload, filtering, permission, pagination and preview behavior.

## Runtime measurements

After the change, React measured: header `x=296,y=24,w=1031,h=95.59`; document surface/layout `x=296,y=139.59,w=1031`; search `x=536,y=139.59,w=751,h=32`; filter row `x=536,y=179.59,w=791,h=32`; filter bar `h=84`. These match the corresponding Vue measurements (Vue header `x=296,y=24,w=1031,h=95.59`, content start `y=139.59`, search/filter rows `32px`, filter bar `84px`).

## Verification

- `pnpm exec tsx --test apps/web/src/documents/page-chrome.test.tsx` — 9/9 passed.
- `pnpm run typecheck:web` — passed.
- Browser runtime paired measurement — passed for the listed geometry at the real authenticated empty/unsupported-parser state.

Responsive, non-empty document cards, upload/preview mutations, and desktop/native rendering remain open and are not claimed by this evidence.
