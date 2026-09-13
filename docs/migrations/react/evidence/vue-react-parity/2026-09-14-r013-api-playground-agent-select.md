# R013 API Playground Agent selector — 2026-09-14

## Vue baseline

`frontend/src/views/integrations/ApiIntegrationSettings.vue:368-377` uses a TDesign `t-select` with filterable options, loading state, placeholder, and agent-load error copy. The selected agent is initialized to the built-in smart-reasoning agent when available, otherwise the first loaded agent.

## React implementation

`apps/web/src/integrations/ApiPlaygroundDrawer.tsx` replaces the native select with a local accessible combobox/listbox. It filters by agent name or id, supports ArrowUp/ArrowDown and Enter selection, closes on Escape or outside click, exposes `aria-busy` during loading, and preserves the localized built-in suffix and selected-agent initialization.

## Verification

- Vue source comparison: complete for selection and interaction semantics.
- DOM tests: loading state, built-in default, filter, Enter selection, Escape close and outside-close path are covered; API Playground suite: 12/12.
- Web TypeScript check: pass.
- Browser computed-style and real-backend agent list evidence: pending.
