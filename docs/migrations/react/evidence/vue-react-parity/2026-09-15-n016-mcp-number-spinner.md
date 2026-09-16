# N016 MCP number input chrome

## Vue baseline

The Vue number-input presentation removes browser spinner affordances and keeps the unit suffix visually passive; the suffix uses the muted placeholder tone.

## React change

The React MCP form applies the same behavior only within `.wk-mcp-form`: Firefox textfield appearance and WebKit inner/outer spin buttons are disabled, while unit labels use the Vue-derived `#98a2b8` muted color. No global input selector was added.

## Verification

- MCP settings tests: 15/15 passed.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.

## Evidence boundary

This is static CSS plus component-test evidence. Authenticated browser computed-style/screenshot capture, live MCP mutation, and Wails/native evidence remain open.
