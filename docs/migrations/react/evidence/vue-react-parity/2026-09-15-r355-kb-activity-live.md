# R355 — Knowledge-base activity settings live parity

## Scope

Authenticated KB settings → 活动记录, using the local `Parity KB Demo` fixture
after the real no-op settings save from R353.

## Vue baseline

`frontend/src/views/knowledge/settings/KnowledgeBaseActivitySettings.vue`
renders a refreshable audit table with action/outcome filters, empty/error and
retry states, incremental loading, and a right-side detail drawer. Live Vue
AX showed the activity title/description, refresh, action and outcome filters,
table columns, four fixture records, and `加载更多`.

## React implementation

Added `apps/web/src/knowledge-bases/KnowledgeBaseActivityPanel.tsx` and mounted
it from the settings editor. The panel now has:

- Vue-derived action/outcome filters and clear-filter feedback;
- refresh, loading, empty, error, retry, and cursor-based load-more states;
- action/result/target/actor/time table columns with localized labels;
- keyboard- and pointer-opened detail drawer using the project Sheet wrapper;
- localized summary fields and raw detail inspection.

The API client activity contract now accepts `after_id`, `action`, `outcome`,
and `limit` query parameters while preserving the old numeric cursor call.

## Live React evidence

React AX at
`http://localhost:5181/platform/knowledge-bases` → `Parity KB Demo` → `设置`
→ `活动记录` showed the matching title, description, refresh button, action
and outcome selectors, table headers, four real backend records, and
`加载更多`. Selecting the first row opened the detail surface; after the
drawer alignment it is rendered as a right-side Sheet with the activity title,
summary fields, and JSON details. No mutation was performed in this section.

## Validation boundary

- Authenticated browser/AX against the local backend fixture: passed.
- API settings tests: 4/4 passed.
- Web typecheck and `git diff --check`: passed.
- Full Web regression/build is run after this slice; provider, Wails, native,
  and production acceptance remain separate evidence layers.

