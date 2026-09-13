# R013 API Playground textarea autosize — 2026-09-14

## Vue baseline

`frontend/src/views/integrations/ApiIntegrationSettings.vue:394-397` uses TDesign textarea autosize with `minRows: 2` and `maxRows: 4`.

## React implementation

`apps/web/src/integrations/ApiPlaygroundDrawer.tsx` starts the textarea at two rows and measures `scrollHeight` after query/open changes. It clamps the rendered height to two through four computed line-heights plus padding, and enables vertical scrolling beyond the maximum.

## Verification

- Vue source comparison: complete.
- React DOM test asserts the two-row minimum; API Playground suite: 10/10.
- Web TypeScript check: pass.
- Browser computed layout and exact Vue pixel comparison: pending because jsdom has no layout engine.
