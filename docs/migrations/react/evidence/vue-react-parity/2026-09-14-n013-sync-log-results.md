# N013 sync-log result details evidence

Date: 2026-09-14

## Vue baseline

The Vue data-source synchronization history distinguishes lifecycle timestamps from result counts, including total, created, updated, deleted, skipped, and failed items, plus an error message when present.

## React implementation

`apps/web/src/data-sources/DataSourcesPage.tsx` now renders `started_at → finished_at`, error details, and all available item counters for each `DataSourceSyncLog`. The values are read from the existing validated API model and do not fabricate completion state.

## Verification

- Data-source form tests: 2/2 passed.
- Web TypeScript check: passed with `--incremental false`.
- Evidence class: static/unit and typecheck only.
- Not claimed: live connector sync, browser comparison, pagination, Wails, iOS, or Android runtime evidence.

## Pagination

The logs surface now requests 20 rows at a time and exposes Previous/Next controls using the API offset. Opening another source resets to offset zero; the Next control is disabled when the returned page is shorter than the page size.
