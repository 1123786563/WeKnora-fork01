# N016 MCP custom headers form item

## Vue baseline

The Vue connection section renders custom headers as a flat form item without a nested card border. Its label row places the add action on the right, followed by the description and the key/value rows with remove actions.

## React change

The React custom-headers fieldset now uses a borderless, compact form-item surface and places the add action in the legend label row. Header key/value editing and removal callbacks are unchanged.

## Verification

- MCP settings tests: 14/14 passed.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.

## Evidence boundary

This is Vue source comparison plus React unit/type/static evidence. Authenticated browser computed-style/screenshot and Wails/native evidence remain open.
