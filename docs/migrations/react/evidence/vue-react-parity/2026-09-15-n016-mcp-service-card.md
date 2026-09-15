# N016 MCP service card typography and footer parity

## Vue baseline

`frontend/src/views/settings/McpSettings.vue` defines a stretched service card, a 14px/20px title row, a 12px/18px description clamped to two lines, and a footer anchored at the bottom of the card.

## React change

`apps/web/src/settings/McpSettingsPanel.tsx` now exposes explicit card anatomy classes. `apps/web/src/settings/settings-wrapper.css` supplies the Vue-derived flex, gap, description clamp, typography, and footer alignment rules within the settings surface.

The service badge and edit/delete/add/tools affordances now use inline stroke SVGs with the same 14px icon geometry role as Vue TDesign icons, including stale warning and chevron states; Unicode glyphs are no longer used for these card controls.

## Verification

- MCP focused tests: 13/13 passed.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.
- Authenticated Chrome AX comparison at 1355×720 confirms React now exposes only the `MCP 服务管理` level-2 heading and its description, matching Vue without the duplicate `MCP服务` wrapper heading.

## Evidence boundary

This slice is source/style comparison plus unit and type evidence. Authenticated same-condition browser screenshots, computed-style capture, synced MCP-server behavior, and Wails/native evidence remain open.
