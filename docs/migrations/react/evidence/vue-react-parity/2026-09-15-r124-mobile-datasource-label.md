# Mobile data-source settings label correction

Date: 2026-09-15  
Scope: `apps/mobile/src/features/knowledge/DataSourcesScreen.tsx`

The connector-settings textarea was announced with the generic “Name” accessibility label. It now uses the existing localized `dataSource.connectorSettingsLabel`, matching the visible field meaning and the Web editor.

Validation: mobile typecheck passed; mobile data-source tests passed 31/31; `git diff --check` passed.
