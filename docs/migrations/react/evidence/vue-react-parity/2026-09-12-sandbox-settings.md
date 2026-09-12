# Sandbox settings parity evidence

Date: 2026-09-12

## Scope

The React Web settings route now uses the shared sandbox-configuration contract for loading, empty and role-gated states, backend tabs/cards, the workspace script kill-switch confirmation, basic create/edit entry, server-backed deletion, inventory inspection, and guarded `force=true` deletion for unverifiable inventory. Delete conflicts are parsed from the shared API error shape and expose live sandbox count, sessions, and agents when the backend provides inventory details. The API client validates the existing sandbox-config routes, DTO envelopes, masked configuration records, inventory conflict codes, and force-delete query.

This remains `implementing`. The Vue multi-step connection/template wizard, template catalog and polling, deep checks, exact field validation, six-locale copy, fixed-viewport screenshots, real backend, Wails, iOS, and Android evidence are not complete.

## Evidence

| Layer | Command / result | Classification |
|---|---|---|
| API contract | `pnpm exec tsx --test packages/api-client/src/sandbox-configurations.test.ts` — 3 passed, 0 failed | focused shared evidence |
| Component SSR | `pnpm exec tsx --test apps/web/src/settings/SandboxSettingsPanel.test.tsx` — 5 passed, 0 failed | focused component evidence |
| Full Web | `pnpm test:web` — 281 passed, 0 failed | Web regression evidence |
| Typecheck | shared and Web TypeScript checks passed | static integration evidence |
| Browser | protected settings route requires authenticated SSO; fixed-viewport recording reached React login only | browser boundary evidence |
| Real backend / native | not run for this slice | missing evidence |

## Remaining work

Do not move R031 to `accepted` until the remaining Vue editor states and protected browser/native/runtime evidence are collected.
