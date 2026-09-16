# N016 MCP editor form labels

## Vue baseline

`McpServiceDialog.vue` uses `.form-label` for ordinary controls: block display, 13px font size, 500 weight, 1.4 line height, and 6px bottom spacing. Inline descriptions use the separate 12px description treatment; checkbox layout is not converted into the ordinary field-label grid.

## React change

The MCP editor form now uses a scoped `.wk-mcp-form` label rule matching those Vue label metrics and excludes the existing checkbox surface from the rule. The change removes the previous form-wide semibold label override inherited from the shadcn migration.

## Verification

- MCP settings tests: 14/14 passed.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.

## Evidence boundary

This is Vue source comparison plus React unit/type/static evidence. Authenticated browser computed-style/screenshot and Wails/native evidence remain open.
