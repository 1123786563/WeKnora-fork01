# React Web renderer / Wails desktop contract audit

Date: 2026-09-16
Scope: `apps/web` React Web renderer and the existing Wails desktop contract. React Native is out of scope.

## Renderer location

There is no `apps/desktop` directory in this checkout. The actual Wails entry is `cmd/desktop`, and [`cmd/desktop/wails.json`](../cmd/desktop/wails.json) sets `frontend:dir` to `../../frontend`. Therefore the packaged Wails renderer is still the Vue app in `frontend/`, not `apps/web`.

This audit does not change `cmd/desktop/wails.json`, Go bindings, or the Vue renderer. Switching the packaged renderer is a separate migration task that requires the full Vue parity and Wails runtime gate.

## Contract findings

| Area | Vue / Wails contract | React state before this change | Result |
| --- | --- | --- | --- |
| API root injection | Wails injects `window.__WEKNORA_API_BASE__` and exposes `go.main.App.GetAPIBaseURL()` | React only read `VITE_API_BASE_URL` and otherwise used the Web origin | Fixed in the Web renderer |
| Wails app binding | Generated binding includes `GetAPIBaseURL`, update, and desktop HTTP preference methods | React had no typed adapter for the binding | API-root adapter added; other methods remain unclaimed until their React settings surface exists |
| Packaged desktop renderer | `cmd/desktop/wails.json` -> `frontend/` | `apps/web` is not selected by Wails | Blocked-by-scope / not changed |

## Implemented renderer change

`apps/web/src/platform/desktop-bridge.ts` resolves the API root in this order:

1. Wails-injected `__WEKNORA_API_BASE__`;
2. generated `go.main.App.GetAPIBaseURL()`;
3. `VITE_API_BASE_URL`;
4. current Web origin.

The React entry now resolves this value before creating the shared API client and scope controller. Trailing slashes are removed and bridge errors fall back safely.

## Evidence

- `node --import tsx --test src/platform/desktop-bridge.test.ts` in `apps/web`: 3 passed, 0 failed.
- `pnpm --filter @weknora/web build`: blocked before Vite build by pre-existing/unrelated type errors in `src/documents/upload-confirm.test.ts` and missing `src/embed/bridge.ts` referenced by `src/embed/bridge.test.ts`.
- No Wails `.app` launch or native WebView interaction was claimed. The packaged renderer remains Vue because of the unchanged Wails config.
