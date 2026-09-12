# Model settings parity evidence

Date: 2026-09-12

## Scope

This slice replaces the generic React model inventory for the settings `models` section with a typed Web panel based on the Vue `ModelSettings.vue` and `ModelEditorDialog.vue` contracts. It covers model-type tabs/counts, tenant-role add/edit/delete controls, built-in visibility, provider loading, remote/local source, base URL validation, embedding dimension validation, context window, vision, concurrency, write-only model credentials, and structured model-in-use details when deletion is rejected by the backend.

It remains `implementing`. The editor now exposes the existing model custom-header and chat thinking-control fields, wires remote chat/embedding/rerank/ASR connection tests through the existing initialization routes, and warns explicitly that local/Ollama model download is not yet ported. Ollama download/search/progress, all provider-specific fields, local-model test/dimension checks, model-usage surface, six-locale copy, fixed-viewport Vue/React screenshots, and authenticated browser/Wails/mobile acceptance remain open.

## Debug panel slice

`apps/web/src/settings/ModelDebugPanel.tsx` now exposes the existing `/api/v1/models/:id/debug` contract for configured Chat, Embedding, Rerank, VLLM, and ASR models. It supports type-specific input, chat parameters, file selection for VLLM/ASR, and structured result/request/observations output. The focused SSR test covers the entry and input surface; API multipart/debug route tests cover the transport. This is not runtime acceptance.

## Evidence

| Layer | Command / result | Classification |
|---|---|---|
| RED | `pnpm exec tsx --test apps/web/src/settings/model-settings.test.ts` initially failed with `ERR_MODULE_NOT_FOUND` before the shared model helper existed | regression proof |
| Pure model contract | `pnpm exec tsx --test apps/web/src/settings/model-settings.test.ts` — 2 passed, 0 failed | focused business evidence |
| Component SSR | `node --import tsx --test apps/web/src/settings/ModelDebugPanel.test.tsx apps/web/src/settings/ModelSettingsPanel.test.tsx` — 3 passed, 0 failed | focused component evidence |
| Model usage conflict UI | `node --import tsx --test apps/web/src/configuration/ModelUsageNotice.test.tsx apps/web/src/configuration/ConfigurationPage.test.ts apps/web/src/settings/ModelSettingsPanel.test.tsx` — 6 passed, 0 failed | shared structured deletion-conflict rendering and existing configuration integration |
| Ollama contract | `pnpm typecheck:web` and existing `packages/api-client/src/settings/index.ts` route contract for status/models/check/download/progress | shared route wiring; runtime Ollama evidence still required |
| Web typecheck | `pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit` — passed | Web static integration evidence |
| Shared/API focused | `pnpm typecheck:shared` plus `packages/api-client/src/configuration.test.ts` — 22 passed, 0 failed | shared contract and route evidence |
| Full Web | `pnpm test:web` — 280 passed, 0 failed | Web regression evidence |
| Browser | protected route requires an authenticated session; the available fixed-viewport recording reached React login and could not enter settings | browser boundary evidence; protected flow missing |
| Real backend / native | not run in this slice | missing evidence |

## Remaining work

Implement the remaining Vue editor states and provider-specific behavior before moving R027 from `implementing` to `review`; complete provider-specific fields, localization, browser/native evidence and a full Web pass are not page acceptance.
