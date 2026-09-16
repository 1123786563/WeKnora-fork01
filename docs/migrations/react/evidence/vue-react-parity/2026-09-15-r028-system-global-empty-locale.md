# React/Vue parity evidence — system global empty locale

- Scope: `apps/web/src/settings/SystemGlobalSettingsPanel.tsx`
- Change: system-settings empty state now uses the same five-locale local copy table as the rest of the panel instead of a Chinese fallback.
- Validation: `pnpm run typecheck:web`, `pnpm run test:web` (891/891), `git diff --check`.
