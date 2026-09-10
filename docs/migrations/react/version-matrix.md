# React 多端迁移版本与兼容矩阵

## Frozen repository inputs

- WeKnora: `5db13a131e10e8ee2105211f665412ebc13bd98e`
- Multica (read-only): `85b1fdbb44fd90aa90ce3353a95c2b1f3d115ddf`
- WeKnora Go module: `module github.com/Tencent/WeKnora`; Wails requirement is `v2.12.0` (`go.mod`), while `cmd/desktop/wails.json` uses schema v2.
- Existing frontend: Vue 3.5, Vite 7, TypeScript 6, Pinia 3, npm lockfile; no React workspace exists yet.
- Existing API description: Swagger/OpenAPI 2.0, 282 paths and 361 operations.

## Reference-only mobile input

- Multica manifest: Expo `~55.0.23`, React `19.2.0`, React Native `0.83.6`.
- `apps/mobile/CLAUDE.md` is stale (describes RN 0.82/React 19.1); manifest/lockfile wins.
- WeKnora must independently lock an Expo-compatible set during T20; no dependency is installed or accepted by this T01 artifact.

## Swagger 2.0 generation trial

- Candidate input: `docs/swagger.yaml`; authority remains `internal/router/routes_*.go`, handler DTOs, middleware and tests.
- Required trial: pin OpenAPI Generator `typescript-fetch` and record Java/runtime versions before T02.
- Current result: not executed in T01 because no generator/toolchain was present in the checkout; therefore generated-client compatibility is **unverified**, not passed.
- Required fixtures: knowledge-base list/create and login, including `null`, omitted fields, unknown enum, `uint64` string precision, 204, 413, non-JSON error and mutation failure.

## Compatibility decisions

| Area | Frozen boundary | Evidence status | Follow-up |
|---|---|---|---|
| Web | React + TypeScript + Vite SPA | decision approved; implementation absent | T02 |
| Desktop | Existing Wails + Go/Lite lifecycle; React renderer later | current Wails path verified | T06/T19 |
| Mobile | Expo + React Native, native UI | no host exists | T20 |
| Backend | Gin REST + SSE + terminal WS unchanged | route registration exists; behavior matrix pending | T01/T02 |
| Embed | separate entry and credential profile | Vue entry exists; React entry absent | T18 |
| Vue retirement | only after T24 acceptance | not eligible | T25 |
