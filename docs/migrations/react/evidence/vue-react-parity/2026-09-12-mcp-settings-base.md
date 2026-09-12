# MCP settings parity evidence — base slice

Date: 2026-09-12

## Scope

The test-result surface now renders tool/resource details and reuses the parent MCP policy mutation path for per-tool enabled and approval controls, matching the Vue behavior boundary without introducing a second policy API.

This slice covers the executable React replacement for `frontend/src/views/settings/McpSettings.vue`: service loading, viewer empty state, admin add entry, service cards, built-in restrictions, edit/delete actions, enable/disable submission locking, dedicated credential transport, metadata refresh/stale handling, tool policy updates, usage persistence, connection test feedback, and OAuth status/authorize/revoke controls.

It does not claim parity for the remaining Vue drawer visuals, six-locale copy, screenshot comparison, or real-backend/Wails/native acceptance. The React editor now includes standard `mcpServers` JSON import, HTTP(S) validation, custom headers, OAuth scopes, and bounded timeout/retry fields; it still explicitly rejects the unsupported stdio remote-editor path. The test-result surface preserves the Vue success/failure state, service description, expandable tool schemas, resource URI/MIME details, and empty state. The metadata tool directory has search, 20-item pagination, description/parameters/schema detail tabs, policy controls, and fail-closed retry state. Metadata refresh/stale handling, usage generation, tool-policy writes, and OAuth status/authorize/revoke controls are now wired, but remain unverified outside focused tests.

## Changes

- `apps/web/src/settings/McpSettingsPanel.tsx` replaces the generic MCP live inventory panel for the settings `mcp` section.
- `apps/web/src/settings/SettingsPage.tsx` routes the `mcp` section to the new panel while preserving the existing role guard and shared client.
- `apps/web/src/styles.css` adds responsive service-card/editor layout styles.
- `apps/web/src/settings/McpSettingsPanel.test.tsx` covers viewer empty state and admin card/action rendering using SSR.
- `packages/api-client/src/configuration.ts` now exposes typed metadata, usage-generation, and tool-policy operations already registered by the MCP backend routes.
- `packages/api-client/src/configuration.test.ts` verifies those paths, encoded tool names, response parsing, and write envelopes.
- `McpServiceDetails` keeps metadata visible when tool-policy loading fails, blocks tool-policy writes against stale metadata, and exposes the backend OAuth authorization lifecycle after an existing service is opened.
- MCP editor save now persists the editable `usage_instructions` field through the existing update route, matching the Vue dialog's generated/manual usage flow.
- MCP editor now preserves Vue's two-step save lifecycle: connection save advances to tools/usage, previous returns to connection editing, and final save requires non-empty usage instructions before updating the saved service.
- Independent review of the prior slice reported 7 Important and 3 Minor findings. The Important findings were repaired in this follow-up: missing default policy rows, usage persistence, stale usage fail-closed behavior, stale-response generation guards, backend-aligned system-admin visibility, bearer/stdio-safe edit hydration, and post-create credential failure recovery.

## Evidence

| Layer | Command / result | Classification |
|---|---|---|
| RED | `pnpm exec tsx --test apps/web/src/settings/McpSettingsPanel.test.tsx` before implementation failed with `ERR_MODULE_NOT_FOUND` for the missing panel | regression proof |
| Component/SSR | `pnpm exec tsx --test apps/web/src/settings/McpSettingsPanel.test.tsx` — 2 passed, 0 failed | focused component evidence |
| Shared/API/component focused | `pnpm exec tsx --test packages/api-client/src/configuration.test.ts apps/web/src/settings/McpSettingsPanel.test.tsx apps/web/src/settings/model-settings.test.ts apps/web/src/settings/ModelSettingsPanel.test.tsx apps/web/src/documents/upload-pipeline.test.ts` — 32 passed, 0 failed | typed and component evidence |
| Web test suite | `pnpm test:web` — 279 passed, 0 failed | Web unit/component regression evidence |
| Shared typecheck | `pnpm typecheck:shared` — 0 | shared static evidence |
| Static | `git diff --check` — 0 | static evidence |
| Web typecheck | `pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit` — passed after preserving the parallel command-palette change and correcting its list response compatibility/type narrowing | Web static integration evidence |
| Browser | `browser-use` at fixed 1355×720 reached the React login page; protected MCP route could not be exercised because no authenticated SSO session was available | real browser boundary evidence; protected flow blocked |
| Real backend | not run in this slice | missing evidence |
| Wails / iOS / Android | not run in this slice | missing evidence |

## Next required work

The metadata/tool/test/usage and OAuth operations are now wired, including structured test/resource details, standard `mcpServers` JSON import, custom headers, OAuth scopes, bounded retry configuration, and the two-step save transition. Exact Vue drawer visuals/localized warning validation, fixed-viewport screenshots, stdio/native behavior, and real-backend role/tenant negative evidence remain before changing these rows to `review` or `accepted`.
