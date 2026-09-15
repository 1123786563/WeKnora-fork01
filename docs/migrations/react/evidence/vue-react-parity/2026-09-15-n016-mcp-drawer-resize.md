# N016 MCP editor drawer resize

## Vue baseline

`McpServiceDialog.vue` delegates to `SettingDrawer` with a 680px default width, 560px minimum, 920px maximum, the `setting-drawer:width:mcp-config-v2` persistence key, viewport clamping, and a vertical resize separator.

## React change

The React MCP drawer now clamps its width to the Vue bounds, restores and persists the same local-storage key, responds to viewport changes, and exposes a vertical separator inside the overlay. Dragging temporarily applies the `col-resize` cursor and disables text selection, then restores the document state on release.

## Verification

- MCP settings tests: 14/14 passed, including min/max/viewport clamp and separator DOM coverage.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.

## Evidence boundary

The resize logic is covered by unit/DOM tests but not yet by an authenticated browser drag capture. Live MCP mutations and Wails/native evidence remain open.
