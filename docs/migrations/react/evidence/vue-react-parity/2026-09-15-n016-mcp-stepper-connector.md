# N016 MCP setup stepper connector

## Vue baseline

The Vue `McpServiceDialog` stepper renders the connector as a 1px line inside the first step. It does not render a textual arrow between steps; step markers and labels carry the navigation affordance.

## React change

Removed the extra textual `→` node from the React stepper so the connector is represented only by the Vue-shaped line. Step activation, completed marker, click-back behavior, disabled state, and save-next transition are unchanged.

## Verification

- MCP settings tests: 13/13 passed.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.

## Evidence boundary

This is Vue source comparison plus React unit/type/static evidence. Authenticated browser screenshot/computed-style and Wails/native evidence remain open.
