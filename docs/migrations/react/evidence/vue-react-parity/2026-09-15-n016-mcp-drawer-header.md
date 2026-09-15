# N016 MCP editor drawer header

## Vue baseline

`McpServiceDialog.vue` uses a 680px default drawer (560px minimum, 920px maximum) whose header has a 32px transport icon, a 15px title, a 12px subtitle/status row, and the setup steps as a separate full-width row. SSE and HTTP Streamable use distinct header-icon colors.

## React change

The React MCP editor drawer now renders the transport icon beside the title/subtitle block, keeps the two-step navigation on its own row, and applies the Vue 32px/9px icon geometry with SSE and HTTP Streamable color treatments. Existing close, step navigation, validation, loading and save behavior remain unchanged.

## Verification

- MCP settings tests: 13/13 passed.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.

## Evidence boundary

This is Vue source comparison plus React unit/type/static evidence. Authenticated post-change browser computed-style/screenshot, drawer resize behavior, live MCP mutations, and Wails/native evidence remain open.
