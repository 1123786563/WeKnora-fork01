# N016 MCP OAuth authorization layout

## Vue baseline

In `McpServiceDialog.vue`, OAuth authorization is a form item with a separate 13px label, a wrapped status/action row, and a 12px/1.5 explanatory hint below it. The authorize and revoke actions use the compact button size and preserve loading/disabled states.

## React change

The React OAuth block now follows the same three-part layout and spacing, uses compact 28px actions, and retains the existing authorization status mapping, popup request, revoke mutation, error surface, and busy guards.

## Verification

- MCP settings tests: 14/14 passed.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.

## Evidence boundary

This is Vue source comparison plus React unit/type/static evidence. Authenticated browser OAuth layout, provider callback, live authorization, and Wails/native evidence remain open.
