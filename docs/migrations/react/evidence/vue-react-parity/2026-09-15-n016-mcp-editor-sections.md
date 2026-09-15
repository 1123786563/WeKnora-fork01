# N016 MCP editor section anatomy

## Vue baseline

`McpServiceDialog.vue` renders the basic, connection, auth, advanced, and usage sections as flat drawer sections: 12px top/16px bottom spacing, 12px inter-item gap, a bottom component-stroke divider, and no rounded card border. Section titles are 13px semibold with a 3px by 14px brand-colored leading bar.

## React change

The React MCP editor replaces the shadcn-style rounded fieldset cards with the Vue section treatment and shared title marker. Existing field order, labels, validation, permissions, metadata gating, and submission callbacks remain unchanged.

## Verification

- MCP settings tests: 14/14 passed.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.

## Evidence boundary

This is Vue source comparison plus React unit/type/static evidence. Authenticated browser computed-style/screenshot, live MCP success/failure flows, and Wails/native evidence remain open.
