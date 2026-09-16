# Data-source form placeholder localization

Date: 2026-09-15  
Scope: `apps/web/src/data-sources/DataSourcesPage.tsx`

The Web data-source editor previously hardcoded English multiline examples for credentials and connector settings. It now uses the existing shared `dataSource.credentialsPlaceholder` and `dataSource.settingsPlaceholder` keys, matching the mobile/shared catalog and preserving locale switching.

Validation: `pnpm typecheck:web` passed; data-source form/resource tests passed 4/4; `git diff --check` passed.
