# React/Vue parity evidence — capability display labels

- Scope: `apps/web/src/settings/PlatformApiKeysPanel.tsx`
- Change: capability values continue to use stable backend identifiers, while form checkboxes and table rows display localized human-readable labels in five locales.
- Validation: `pnpm run typecheck:web`, `pnpm run test:web` (891/891), `git diff --check`.
- Limitation: runtime create/revoke evidence remains environment dependent.
