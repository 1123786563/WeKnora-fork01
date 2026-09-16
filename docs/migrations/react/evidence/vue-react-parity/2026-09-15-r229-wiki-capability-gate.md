# R229 Wiki capability gate

## Runtime baseline

Same authenticated session (`paritytester`, `parity-test@local.dev`), viewport 1355x720, DPR2, zh-CN, and knowledge base `Parity KB Demo` were used against the local Vue server (`:5173`) and React server (`:5181`). The backend record has `indexing_strategy.wiki_enabled=false`.

Vue's real detail entry is `/platform/knowledge-bases/:kbId?tab=wiki` (Vue has no `/knowledgeBase/:kbId/wiki` route). Because the capability is disabled, Vue stays on the documents surface, does not render Wiki controls, and does not show a Wiki API error. The React detail query previously rendered the Wiki page anyway, including the backend error and contributor-only create form.

## Change

`apps/web/src/knowledge/wiki-route.ts` adds the explicit capability predicate and historical-entry fallback path. `apps/web/src/main.tsx` now probes the KB settings before rendering either React Wiki entry; a disabled capability renders the existing documents page and preserves the normal document-detail navigation. Lookup failure keeps the existing Wiki error surface instead of masking an unavailable backend.

## Verification

- Focused route contract: `pnpm exec tsx --test apps/web/src/knowledge/wiki-route.test.ts` — 2/2 passed.
- Web typecheck: `pnpm run typecheck:web` — passed.
- Browser runtime: React `/platform/knowledge-bases/:kbId?tab=wiki` now renders the same documents surface as Vue; `hasWikiError=false`, `hasCreate=false`, and the route is still protected by the existing shell/auth gate.

This closes the observed disabled-capability behavior difference for the web entry. Wiki-enabled KB visual/interaction comparison, protected CRUD/conflict flows, responsive behavior, and desktop/native evidence remain open.
