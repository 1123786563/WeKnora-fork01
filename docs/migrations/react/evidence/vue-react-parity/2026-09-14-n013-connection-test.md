# N013 data-source connection test evidence

Date: 2026-09-14

## Vue baseline

The Vue data-source flow distinguishes credential/connection validation from a synchronization run. A saved connector can be tested independently, and validation feedback must not be presented as completed synchronization.

## React implementation

`apps/web/src/data-sources/DataSourcesPage.tsx` adds a `Test connection` action for saved sources. It uses `client.dataSources.validate`, participates in the existing in-flight action guard, reports a dedicated success message, and leaves the source list untouched on validation success. Sync, pause, resume, and log actions remain separate.

## Verification

- Data-source form tests: 2/2 passed.
- Web TypeScript check: passed with `--incremental false`.
- Evidence class: static/unit and typecheck only.
- Not claimed: browser interaction, real connector validation, resource browsing, full log drawer parity, Wails, iOS, or Android runtime evidence.

The visible connection-test label and success feedback, plus the surrounding page headings/actions, now resolve through the shared data-source translation table rather than English-only literals.
