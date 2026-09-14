# React/Vue parity evidence — API key capability labels

- Scope: `apps/web/src/settings/PlatformApiKeysPanel.tsx`
- Change: capability identifiers remain unchanged in API payloads, while create controls and existing-key rows display localized labels in five locales.
- Validation: `pnpm run typecheck:web`, `pnpm run test:web` (891/891), `git diff --check`.
- Limitation: authenticated create/revoke runtime evidence remains pending.
