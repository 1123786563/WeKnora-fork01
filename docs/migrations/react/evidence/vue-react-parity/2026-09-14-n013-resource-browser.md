# N013 data-source resource browser evidence

Date: 2026-09-14

## Vue baseline

The Vue data-source editor loads selectable resources lazily, shows a root list, opens expandable children, keeps a breadcrumb-like navigation context, and presents loading/error/empty states inside the resource area.

## React implementation

`apps/web/src/data-sources/DataSourcesPage.tsx` adds `Browse resources` for saved sources. The resource surface calls `client.dataSources.resources` with an optional parent id, renders root and child rows, keeps a trail for Back navigation, and handles loading/error/empty states without fabricating resources.

## Verification

- Data-source form tests: 2/2 passed.
- Web TypeScript check: passed with `--incremental false`.
- Evidence class: static/unit and typecheck only.
- Not claimed: resource checkbox selection persistence, live connector resources, browser comparison, Wails, iOS, or Android runtime evidence.
