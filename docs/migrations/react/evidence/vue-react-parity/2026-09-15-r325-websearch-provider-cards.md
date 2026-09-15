# WebSearch provider-card parity — 2026-09-15 R325

## Scope

Only `apps/web/src/settings/ResourceSettingsPanel.tsx` and its dedicated test were changed for this slice. Storage, VectorStore, Vue sources, shared settings CSS, permissions, API client, i18n catalogs, and parity ledgers were not changed.

## Vue reference

`frontend/src/views/settings/WebSearchSettings.vue:10-87` defines the provider-card anatomy: provider badge, provider name, provider type, optional description, optional proxy URL, provider actions, and an admin-only add card.

## Implemented slice

- WebSearch rows now render Vue-shaped `provider-card` / `provider-card__*` structure with provider-derived badge, type, description, and `parameters.proxy_url` metadata.
- Edit and delete actions retain the existing API handlers and busy-state/confirmation behavior, with accessible provider-specific `aria-label` values; the add card resets the existing editor without changing the API or i18n contract.
- Storage and VectorStore continue through the original generic resource-row renderer.

## Verification

| Check | Result |
|---|---|
| `pnpm exec tsx --test apps/web/src/settings/ResourceSettingsPanel.test.tsx` | 2/2 passed |
| `pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit` | exit 0 |
| `git diff --check` | exit 0 |

## Evidence boundary

This is source/static plus focused server-render test evidence. It does not establish authenticated browser pixel comparison, real-backend provider CRUD/test callbacks, responsive behavior, or Wails/native acceptance.
