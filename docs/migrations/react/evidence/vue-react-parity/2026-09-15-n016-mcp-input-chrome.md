# N016 MCP form control chrome

## Vue baseline

The Vue MCP dialog uses the shared TDesign input/select chrome at roughly 32px height, 4px radius, 9px horizontal padding, and 7px vertical padding. The textarea uses the same border/radius family and remains scoped to the drawer form.

## React change

The React MCP form now explicitly scopes 32px input/select height, 4px radius, 9px/7px padding, and matching border/background/font treatment to `.wk-mcp-form`, preventing shadcn default control geometry from leaking into this dialog.

## Verification

- MCP settings tests: 14/14 passed.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.

## Evidence boundary

This is Vue source comparison plus React unit/type/static evidence. Authenticated browser computed-style/screenshot, keyboard/focus capture, and Wails/native evidence remain open.
