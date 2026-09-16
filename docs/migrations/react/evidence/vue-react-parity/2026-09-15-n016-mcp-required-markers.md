# N016 MCP required field markers

## Vue baseline

Vue marks the name, transport type, service URL, and usage-instructions labels with `.required::before`: a red `*` followed by 4px spacing, while optional fields remain unmarked.

## React change

React now renders the same scoped required marker for those four fields using a label-text wrapper, without changing native `required` validation or submission behavior.

## Verification

- MCP settings tests: 14/14 passed.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.

## Evidence boundary

This is Vue source comparison plus React unit/type/static evidence. Authenticated browser computed-style/screenshot and Wails/native evidence remain open.
