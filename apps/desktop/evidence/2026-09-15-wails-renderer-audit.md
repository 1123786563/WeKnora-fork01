# React desktop / Wails renderer audit

Date: 2026-09-16

Scope: `apps/desktop/**` only. Vue remains the behavioral reference. Mobile was not inspected for changes and was not modified.

## Findings and disposition

| Boundary | Evidence | Disposition |
| --- | --- | --- |
| WebView viewport | `apps/desktop/index.html` declares `width=device-width, initial-scale=1.0`; `runtime.ts` applies 1440x900 with a 1024x680 minimum through the Wails runtime API. | Covered by focused test. The Go shell still declares 1280x800 in `cmd/desktop/main.go`; changing that file was outside this task's allowed scope. |
| API bridge | The shared React entry reads `window.__WEKNORA_API_BASE__` during module evaluation. Wails' generated binding is asynchronous (`GetAPIBaseURL(): Promise<string>`), so the desktop entry now awaits `App.GetAPIBaseURL()` before importing the shared renderer and accepts only HTTP(S). | Repaired and covered by focused test, including rejected and invalid-scheme Promises. |
| External URLs | Vue/Wails uses `runtime.BrowserOpenURL`; the React desktop entry previously relied on the later Go `OnDomReady` script. The desktop entry now installs an early `window.open` bridge and rejects non-HTTP(S) schemes. | Repaired and covered by focused test. Direct anchor interception remains additionally present in the Go shell's DomReady script. |
| Deep links | Local legacy paths are normalized before the shared route bootstrap; protocol and network-path values are rejected by `isSafeDesktopDeepLink`. | Covered by focused tests. Wails OS protocol delivery/second-instance forwarding was not runnable here. |
| Credential bridge | React has a desktop credential storage abstraction, but the current Go `App` has no `GetCredential`/`SetCredential`/`DeleteCredential` (or keychain equivalent) binding. The Wails config points at `../../apps/desktop`, while the generated bindings remain under the legacy Vue tree. | `blocked-env` / integration gap. No claim of secure desktop credential persistence is made; no Go or generated-binding changes were made because they are outside the requested `apps/desktop/**` scope. |

## Verification

```text
pnpm --filter @weknora/desktop-renderer test       8 pass, 0 fail
pnpm --filter @weknora/desktop-renderer typecheck  exit 0
pnpm --filter @weknora/desktop-renderer build      exit 0
```

The production build reports the existing CSS `@import` ordering warning and large-chunk advisory. The `wails` CLI is not installed in this environment, so native Wails build/launch, WebView interaction, OS external-browser handoff, credential storage, and deep-link delivery remain `blocked-env`.
