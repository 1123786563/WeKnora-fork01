# React/Vue parity evidence — system setting labels

- Scope: `apps/web/src/settings/SystemGlobalSettingsPanel.tsx`
- Change: mapped known system setting keys to human-readable labels in all five supported locales; unknown keys retain the existing readable fallback.
- Validation: `pnpm run typecheck:web`, `pnpm run test:web` (891/891), `git diff --check`.
