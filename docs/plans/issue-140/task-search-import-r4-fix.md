# Search URL import receipt identity repair (OCR round 4)

## Scope and finding

- Issue: #140, bounded round 4 repair for `CareerSearchPage.importResult`.
- Finding source: `docs/plans/issue-140/ocr/round4-highrisk-analysis.md`, SearchPage item at lines 51: `[VALID] bug·medium` — `imported.requestId` was not compared to the current import attempt before the receipt was stored and its evidence link rendered.
- Corroborating review evidence: `docs/plans/issue-140/task-career-web-medium-ocr-r4-review.md` records SearchPage as excluded from that review because Task 2 owned it; `docs/plans/issue-140/ocr/ocr-round-4.md` and `ocr-round-4-resume.md` retain the same request-ID finding.
- Starting HEAD: `50932d609f3a81666ec6efd5d6ae8ba8ebefae64`.
- Ownership: `SearchPage.tsx`, its focused test, and this report only.

## Change

`importResult` now compares the returned URL-import receipt `requestId` with the exact ID sent for that row. A mismatch is treated as a definite invalid response: it clears the row's retained attempt ID and any receipt, exits busy/uncertain state, and shows a row-level alert. The alien receipt is never accepted, so no opportunity evidence link is rendered. A later user-initiated import starts with a fresh request ID.

The valid-success fixtures now echo the request ID received by `importUrl`, matching the API contract.

## TDD and verification

| Command | Result |
|---|---|
| `node --import tsx --test apps/web/src/career/SearchPage.test.tsx` before implementation | Exit 1 as expected: 17 passed, 1 failed; the new mismatch case found no row-level alert. |
| `node --import tsx --test apps/web/src/career/SearchPage.test.tsx` after implementation | Exit 0; 18 passed, 0 failed. The mismatch test verifies a visible row alert, no evidence link, no uncertain/retry status, and one import attempt. Existing valid success and uncertain retry tests pass. |
| `pnpm typecheck:web` | Exit 0; `tsc -p tsconfig.json --noEmit` passed for the web app. |
| `git diff --check -- apps/web/src/career/SearchPage.tsx apps/web/src/career/SearchPage.test.tsx docs/plans/issue-140/task-search-import-r4-fix.md` | Exit 0; no whitespace errors. |

The alert uses the existing accessible `role="alert"` row pattern. This fix changes no layout or responsive styles; browser visual checks are not applicable to the request/receipt state mismatch.

## Limits

- This task covers only the URL-import response identity mismatch in SearchPage.
- Other worktree changes were present in miniprogram, backend, and round 4 report files; they were not modified or staged by this task.
- No independent review or OCR was run by this task; the parent owns those gates.
