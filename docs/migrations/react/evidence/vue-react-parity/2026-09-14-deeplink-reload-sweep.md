# 2026-09-14 deep-link / reload / back-forward sweep

Objective §四.1 requires direct access, refresh, and history traversal
checks. Script: .parity-tools/deeplink-reload-sweep.cjs (gitignored).
Coverage: 8 routes x 2 ends x {direct deep link, reload in place,
back, forward} = 16 checks, all stable (deepAnchor === reloadAnchor,
no drift, no redirect surprises). Results:
docs/migrations/react/evidence/vue-react-parity/deeplink-reload-20260914/deeplink-reload-results.json

## Observations
- Legacy /platform/knowledge-search deep link works on both ends
  (Vue: knowledge-base alias; React: cmdk consumption + redirect).
- React's document-order first heading on platform routes is the shell
  sessions h2 (我的对话); heading ORDER relative to the page h1 differs
  from Vue (cosmetic, a11y-hierarchy nit — registered, not blocking).
- R009 anatomy WIP was live in the React tree during this sweep; all
  routes still rendered and survived reload/back-forward.

## Limits
- Anchor-based check (first h1/h2 text) — not a full visual diff;
  pairing with accept-batch screenshots covers visuals.
- Query-param state (settings ?section=) verified via deepAnchor
  stability for two settings sections; remaining sections inferable.
