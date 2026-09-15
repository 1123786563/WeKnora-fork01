# R351 — Document-card processing copy live parity

- Runtime: authenticated Chrome session, zh-CN, same local backend and `Parity KB Demo` on Vue `:5180` and React `:5181`.
- Fixture state: the manual document was still in an in-flight parse state (`processing`) during both inspections.

## Vue baseline

- `frontend/src/views/knowledge/components/DocumentCardView.vue` maps `pending`, `processing`, and `finalizing` through `inFlightCardStatusText`.
- For `pending`/`processing`, the card says `解析中...`; for `finalizing`, it distinguishes summary generation from finalization.
- The status filter option may say `处理中`, but that is not the card copy.

## React discrepancy and repair

- React `documentStatus` used the filter/status key `knowledgeBase.parseStatusProcessing`, rendering `处理中` on the document card.
- `apps/web/src/documents/KnowledgeDocumentsPage.tsx` now uses `knowledgeBase.parsingInProgress` for `pending`/`processing`, and uses Vue-compatible summary/finalizing branching.
- The card header now also uses a Tailwind spinner + trace button for in-flight parse states, matching Vue's green `card-analyze` treatment instead of the generic warning paragraph.
- Added regression coverage in `apps/web/src/documents/page-chrome.test.tsx` for pending, processing, finalizing, and finalizing-with-summary states.

## Evidence

- Vue AX tree showed the document card's in-flight label `解析中...`.
- After Vite hot reload, React AX tree showed the same `解析中...` label for the same document and backend state.
- No upload, reparse, cancel, delete, or save action was submitted.

## Verification boundary

- Focused document page-chrome tests: 18/18 passed.
- This proves the card-state copy branch and live rendering only; upload confirmation behavior, real parse success/failure, Wails, and native acceptance remain open.
