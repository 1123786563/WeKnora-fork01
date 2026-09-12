# MCP settings parity evidence — base slice

Date: 2026-09-12

## Scope

This slice covers the first executable React replacement for `frontend/src/views/settings/McpSettings.vue`: service loading, viewer empty state, admin add entry, service cards, built-in restrictions, edit/delete actions, enable/disable submission locking, and dedicated credential transport for a newly entered API key.

It does not claim parity for the Vue two-step `McpServiceDialog`, metadata refresh/stale state, paginated tool policies, test result rendering, code import, usage generation, OAuth popup/callback, six-locale copy, screenshot comparison, or real-backend/Wails/native acceptance.

## Changes

- `apps/web/src/settings/McpSettingsPanel.tsx` replaces the generic MCP live inventory panel for the settings `mcp` section.
- `apps/web/src/settings/SettingsPage.tsx` routes the `mcp` section to the new panel while preserving the existing role guard and shared client.
- `apps/web/src/styles.css` adds responsive service-card/editor layout styles.
- `apps/web/src/settings/McpSettingsPanel.test.tsx` covers viewer empty state and admin card/action rendering using SSR.

## Evidence

| Layer | Command / result | Classification |
|---|---|---|
| RED | `pnpm exec tsx --test apps/web/src/settings/McpSettingsPanel.test.tsx` before implementation failed with `ERR_MODULE_NOT_FOUND` for the missing panel | regression proof |
| Component/SSR | `pnpm exec tsx --test apps/web/src/settings/McpSettingsPanel.test.tsx` — 2 passed, 0 failed | focused component evidence |
| Static | `git diff --check` — 0 | static evidence |
| Web typecheck | `pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit` — blocked by pre-existing dirty `apps/web/src/platform/command-palette-search.ts` errors at lines 272, 304 | integration blocked |
| Browser / real backend | not run in this slice | missing evidence |
| Wails / iOS / Android | not run in this slice | missing evidence |

## Next required work

Extend the panel against the existing typed MCP API with metadata get/refresh, tool approval/enabled mutations, test result/resource/tool rendering, two-step save gating, OAuth status/authorize/revoke, usage instructions, code import, and Vue-derived localized validation/feedback. Then collect fixed-viewport Vue/React browser screenshots and real-backend role/tenant negative evidence before changing these rows to `review` or `accepted`.
