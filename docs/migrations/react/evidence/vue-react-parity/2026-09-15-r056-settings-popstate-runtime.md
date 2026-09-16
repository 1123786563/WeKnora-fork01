# R056 Settings popstate runtime

- Runtime: authenticated React Web.
- Navigation: opened `/platform/knowledge-bases`, then `/platform/settings?section=mcp`, then used browser Back.
- Observed result: browser returned to `/platform/knowledge-bases` and restored the protected shell/list after the route transition.
- No settings or backend data were changed.
