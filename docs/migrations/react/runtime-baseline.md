# T01 运行基线

- Captured at: 2026-09-10
- WeKnora HEAD: `0fefc0310810c25fbb58b8f5f704edd6c0056bd9`
- Multica HEAD (read-only): `85b1fdbb44fd90aa90ce3353a95c2b1f3d115ddf`
- Vue SFC count: `200` (plan/inventory said 199; current is 200)
- API TS count under `frontend/src/api`: `33`
- Locales: `zh-CN`, `en-US`, `ja-JP`, `ko-KR`, `ru-RU`, plus embed locale resources
- Legacy URL sources: `frontend/src/router/index.ts`; route matrix records the retained entries
- Desktop data/runtime sources: `cmd/desktop/main.go`, `cmd/desktop/prefs.go`, `cmd/desktop/update.go`; Wails frontend is currently `../../frontend`
- API description: Swagger 2.0, 282 paths/361 operations

## Evidence status

| Layer | Result | Evidence |
|---|---|---|
| Static inventory | collected | route/API/reuse/version matrices in this directory |
| Mock transport | not applicable to T01 | T02 |
| Existing frontend build/test | not run in T01 | dependency installation and baseline command are still pending |
| Real backend smoke | not run | requires a safe isolated backend and credentials; no production data used |
| Wails installed package | not run | T19; browser/WebView preview is not package evidence |
| iOS/Android native | not run | T20-T23; no Expo host exists yet |
| Core screenshots/performance | not collected | requires running frontend and a stable fixture/backend |

## Known baseline blockers

1. `frontend/node_modules` and root React workspace are absent; no build claim is made.
2. The three authoritative input documents are currently untracked user files; this ledger must not overwrite or clean them.
3. Swagger is a 2.0 document inventory, not proof of handler behavior; every generated row remains explicitly marked for handler/DTO/permission review.
4. Real smoke and native/Wails package evidence require environment inputs not present in this T01 run.
