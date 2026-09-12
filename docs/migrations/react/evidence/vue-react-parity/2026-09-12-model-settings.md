# Model settings parity evidence

Date: 2026-09-12

## Scope

This slice replaces the generic React model inventory for the settings `models` section with a typed Web panel based on the Vue `ModelSettings.vue` and `ModelEditorDialog.vue` contracts. It covers model-type tabs/counts, tenant-role add/edit/delete controls, built-in visibility, provider loading, remote/local source, base URL validation, embedding dimension validation, context window, vision, concurrency, and write-only model credentials.

It remains `implementing`. The editor now exposes the existing model custom-header and chat thinking-control fields and warns explicitly that local/Ollama model download is not yet ported. Ollama download/search/progress, all provider-specific fields, remote test/embedding dimension checks, debug/usage surfaces, six-locale copy, fixed-viewport Vue/React screenshots, and authenticated browser/Wails/mobile acceptance remain open.

## Evidence

| Layer | Command / result | Classification |
|---|---|---|
| RED | `pnpm exec tsx --test apps/web/src/settings/model-settings.test.ts` initially failed with `ERR_MODULE_NOT_FOUND` before the shared model helper existed | regression proof |
| Pure model contract | `pnpm exec tsx --test apps/web/src/settings/model-settings.test.ts` — 2 passed, 0 failed | focused business evidence |
| Component SSR | `pnpm exec tsx --test apps/web/src/settings/ModelSettingsPanel.test.tsx` — 2 passed, 0 failed | focused component evidence |
| Web typecheck | `pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit` — passed | Web static integration evidence |
| Browser | protected route requires an authenticated session; the available fixed-viewport recording reached React login and could not enter settings | browser boundary evidence; protected flow missing |
| Real backend / native | not run in this slice | missing evidence |

## Remaining work

Implement the remaining Vue editor states and provider-specific behavior before moving R027 from `implementing` to `review`; no static pass is treated as page acceptance.
