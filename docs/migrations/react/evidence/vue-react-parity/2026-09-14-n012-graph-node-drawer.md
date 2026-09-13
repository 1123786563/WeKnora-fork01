# N012 knowledge graph node detail evidence

Date: 2026-09-14

## Vue baseline

`frontend/src/views/knowledge/wiki/WikiBrowser.vue` renders a graph node drawer with the selected page title, type/version information, page content, loading/error states, close behavior, and actions for ego/bloom neighbor expansion.

## React implementation

`apps/web/src/knowledge/KnowledgeGraphPage.tsx` now opens `.wk-graph-drawer` when a graph node or node-list action is activated. It fetches the page through `client.wiki.get`, renders loading/error/content states, supports close, and keeps the existing ego graph request as the explicit expansion action.

## Verification

- Focused graph and permission tests: 8/8 passed.
- Web TypeScript check: passed with `--incremental false`.
- Evidence class: static/unit and typecheck only.
- Not claimed: Vue computed-style comparison, authenticated browser interaction, real-backend graph data, Wails, iOS, or Android runtime evidence.

## Remaining N012 gaps

Remote graph search, legend/help/status-card parity, frontier growth/bloom behavior, and browser/real-backend/platform evidence remain open.
