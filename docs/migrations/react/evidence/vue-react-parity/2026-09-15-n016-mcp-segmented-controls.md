# N016 MCP segmented transport and auth controls

## Vue baseline

`McpServiceDialog.vue` uses compact `source-options` groups for transport and authentication: 3px container padding, 4px group gap, 28px option height, 5px/12px option padding, 6px radius, and active brand border/background treatment. These controls are button-based rather than native selects.

## React change

React now renders transport and auth as accessible `radiogroup`/`radio` button groups with Vue-matched geometry, active state, hover/focus feedback, and transport icons. State updates still flow through the existing draft and payload logic.

## Verification

- MCP settings tests: 14/14 passed, including segmented transport structure and stdio coercion.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.

## Evidence boundary

This is Vue source comparison plus React unit/type/static evidence. Authenticated browser interaction/computed-style, real MCP mutations, and Wails/native evidence remain open.
