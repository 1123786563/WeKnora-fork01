# N012 graph remote search evidence

Date: 2026-09-14

## Vue baseline

`WikiBrowser.vue` debounces graph search input, queries the Wiki page list, shows title/slug options, and loads the selected page's graph state. Empty and stale responses are suppressed by the active search sequence.

## React implementation

`KnowledgeGraphPage.tsx` keeps the graph query filter, adds a 250ms debounced `client.wiki.list` search for input of at least two characters, cancels stale timers/results, and renders keyboard-focusable scoped suggestions. Selecting a suggestion opens the node detail surface and requests ego neighbors.

## Verification

- Graph helper tests: 3/3 passed.
- Web TypeScript check: passed with `--incremental false`.
- Evidence class: static/unit and typecheck only.
- Not claimed: browser interaction, Vue computed-style comparison, real-backend graph data, Wails, iOS, or Android runtime evidence.
