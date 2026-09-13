# N007 graph action loading parity — 2026-09-14

## Vue baseline

`frontend/src/views/knowledge/settings/GraphSettings.vue` keeps independent `tagFabring` and `textFabring` flags around `fabriTag` and `fabriText`. The corresponding TDesign buttons are disabled while their request is pending and show loading; both flags are cleared in `finally`. Entity-relation extraction has its separate `extracting` flag.

## React change

`apps/web/src/documents/KnowledgeDocumentsPage.tsx` now mirrors the independent tag/text busy flags. The two generation buttons are disabled when the LLM is unavailable or their own request is in flight; `runAction` clears the flag in `finally`, including rejected requests. Entity-relation extraction keeps its existing separate busy state.

## Verification

- Vue source comparison: complete for the three GraphSettings actions.
- React focused graph tests: pass as part of `upload-confirm-dialog.test.tsx` graph section cases.
- Web TypeScript check: pass.
- Full Web test suite: 813/813 pass.
- Browser/real-backend evidence: not collected for a successful admin LLM request; the current local owner session is not system-admin and graph database capability is disabled, so this item remains review.
